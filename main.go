package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type Owner struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

type Repo struct {
	Name          string `json:"name"`
	Owner         Owner  `json:"owner"`
	NameWithOwner string `json:"nameWithOwner"`
	IsArchived    bool   `json:"isArchived"`
	UpdatedAt     string `json:"updatedAt"`
	Parent        struct {
		NameWithOwner    string `json:"nameWithOwner"`
		DefaultBranchRef struct {
			Name   string `json:"name"`
			Target struct {
				Oid string `json:"oid"`
			} `json:"target"`
		} `json:"defaultBranchRef"`
	} `json:"parent"`
	DefaultBranchRef struct {
		Name   string `json:"name"`
		Target struct {
			Oid string `json:"oid"`
		} `json:"target"`
	} `json:"defaultBranchRef"`
}

type PullRequestInfo struct {
	HeadRepository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"headRepository"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Url    string `json:"url"`
}

type CommitComparison struct {
	AheadBy  int `json:"ahead_by"`
	BehindBy int `json:"behind_by"`
}

func showSpinner(done chan bool) {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r") // Clear the spinner
			return
		default:
			fmt.Printf("\r%s Fetching forks", spinner[i])
			i = (i + 1) % len(spinner)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func getReposWithOpenPRs() (map[string][]PullRequestInfo, error) {
	// GraphQL query to get all open PRs
	query := `
		query {
		  viewer {
			  pullRequests(states: [OPEN], first: 100) {
				  nodes {
					  headRepository {
						  nameWithOwner
					  }
					  number
					  title
					  url
					}
				}
			}
		}
	`

	cmd := exec.Command("gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error fetching open PRs: %v\nOutput: %s", err, string(out))
	}

	// Parse the GraphQL response
	type Response struct {
		Data struct {
			Viewer struct {
				PullRequests struct {
					Nodes []PullRequestInfo `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"viewer"`
		} `json:"data"`
	}

	var resp Response
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("error parsing GraphQL response: %v", err)
	}

	// Create a map of repos with open PRs and their info
	reposWithPRs := make(map[string][]PullRequestInfo)
	for _, node := range resp.Data.Viewer.PullRequests.Nodes {
		if node.HeadRepository.NameWithOwner != "" {
			reposWithPRs[node.HeadRepository.NameWithOwner] = append(
				reposWithPRs[node.HeadRepository.NameWithOwner],
				PullRequestInfo{
					HeadRepository: node.HeadRepository,
					Number:         node.Number,
					Title:          node.Title,
					Url:            node.Url,
				},
			)
		}
	}

	return reposWithPRs, nil
}

func getForks() ([]Repo, error) {
	// GraphQL query to get all forks with pagination
	query := `
		query($after: String) {
			viewer {
				repositories(first: 100, after: $after, isFork: true, orderBy: {field: UPDATED_AT, direction: DESC}) {
					nodes {
						name
						nameWithOwner
						updatedAt
						isArchived
						owner {
							login
							id
						}
						parent {
							nameWithOwner
							defaultBranchRef {
								name
								target {
									oid
								}
							}
						}
						defaultBranchRef {
							name
							target {
								oid
							}
						}
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}
	`

	var forks []Repo
	var cursor string
	hasNextPage := true

	for hasNextPage {
		// Build the command with the cursor
		args := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", query)}
		if cursor != "" {
			args = append(args, "-f", fmt.Sprintf("after=%s", cursor))
		}
		cmd := exec.Command("gh", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("error fetching forks: %v\nOutput: %s", err, string(out))
		}

		// Parse the GraphQL response
		type Response struct {
			Data struct {
				Viewer struct {
					Repositories struct {
						Nodes    []Repo `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"repositories"`
				} `json:"viewer"`
			} `json:"data"`
		}

		var resp Response
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("error parsing GraphQL response: %v", err)
		}

		// Append the forks from this page
		forks = append(forks, resp.Data.Viewer.Repositories.Nodes...)

		// Update pagination info
		hasNextPage = resp.Data.Viewer.Repositories.PageInfo.HasNextPage
		if hasNextPage {
			cursor = resp.Data.Viewer.Repositories.PageInfo.EndCursor
		}
	}

	return forks, nil
}

func getCommitComparison(fork Repo) (*CommitComparison, error) {
	if fork.Parent.NameWithOwner == "" || fork.Parent.DefaultBranchRef.Name == "" || fork.DefaultBranchRef.Name == "" {
		return nil, fmt.Errorf("missing required repository information")
	}

	// Use gh api to get the comparison between the fork and its parent
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/compare/%s...%s:%s",
			fork.Parent.NameWithOwner,
			fork.Parent.DefaultBranchRef.Name,
			fork.Owner.Login,
			fork.DefaultBranchRef.Name,
		),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error comparing repositories: %v\nOutput: %s", err, string(out))
	}

	var comparison CommitComparison
	if err := json.Unmarshal(out, &comparison); err != nil {
		return nil, fmt.Errorf("error parsing comparison response: %v", err)
	}

	return &comparison, nil
}

var rootCmd = &cobra.Command{
	Use:   "gh-fork-cleanup",
	Short: "Clean up your GitHub forks",
	Long: `A CLI tool to help you clean up your GitHub forks.
It shows you all your forks, highlighting those that haven't been updated recently
and allows you to delete them if they don't have any open pull requests.`,
	Run: cleanupForks,
}

func init() {
	rootCmd.Flags().BoolP("skip-confirmation", "s", false, "Skip confirmation for forks with open pull requests")
	rootCmd.Flags().BoolP("force", "f", false, "It will automatically delete all forks. Be careful when using this option.")
}

func cleanupForks(cmd *cobra.Command, args []string) {
	// Start spinner
	done := make(chan bool)
	go showSpinner(done)

	// Get flags
	force, _ := cmd.Flags().GetBool("force")
	skipConfirmation, _ := cmd.Flags().GetBool("skip-confirmation")

	// Get all repos with open PRs
	color.New(color.FgBlue).Println("Fetching repositories with open pull requests...")
	reposWithPRs, err := getReposWithOpenPRs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Fetch all forks using GraphQL
	forks, err := getForks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Stop spinner
	done <- true

	if len(forks) == 0 {
		fmt.Println("No forked repositories found.")
		os.Exit(0)
	}

	color.New(color.FgCyan, color.Bold).Printf("📦 Found %d forks\n", len(forks))
	scanner := bufio.NewScanner(os.Stdin)
	for _, fork := range forks {
		fmt.Print("\n")
		color.New(color.FgGreen, color.Bold).Printf("📂 Repository: %s\n", fork.Name)
		color.New(color.FgBlue).Printf("   🔄 Forked from: %s\n", fork.Parent.NameWithOwner)
		if fork.IsArchived {
			color.New(color.FgRed).Println("   📦 This repository is archived")
		}

		// Show commit comparison information
		if comparison, err := getCommitComparison(fork); err == nil {
			if comparison.AheadBy > 0 || comparison.BehindBy > 0 {
				color.New(color.FgBlue).Printf("   📊 Commits: %d ahead, %d behind\n",
					comparison.AheadBy,
					comparison.BehindBy,
				)
			}
		}

		// Show PR information upfront
		if prs, hasPRs := reposWithPRs[fork.NameWithOwner]; hasPRs {
			color.New(color.FgRed).Printf("   ⚠️ Has %d open pull request(s):\n", len(prs))
			for _, pr := range prs {
				color.New(color.FgYellow).Printf("      #%d: %s\n", pr.Number, pr.Title)
				color.New(color.FgBlue).Printf("      URL: %s\n", pr.Url)
			}
		}
		color.New(color.FgYellow).Printf("   📅 Last updated: %s\n", fork.UpdatedAt)
		if !force {
			color.New(color.FgMagenta).Print("❔ Delete this repository? (y/n/o to open in browser, default n): ")
			scanner.Scan()
			answer := strings.ToLower(strings.TrimSpace(scanner.Text()))

			if answer == "o" {
				repoURL := fmt.Sprintf("https://github.com/%s", fork.NameWithOwner)
				openCmd := exec.Command("xdg-open", repoURL)
				if err := openCmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Error opening URL: %v\n", err)
				}
				// Ask again after opening the URL
				color.New(color.FgMagenta).Print("❔ Delete this repository? (y/n, default n): ")
				scanner.Scan()
				answer = strings.ToLower(strings.TrimSpace(scanner.Text()))
			}

			if answer != "y" {
				color.New(color.FgBlue).Printf("⏭️  Skipping %s...\n", fork.Name)
				continue
			}

			// Double confirm if there are open PRs and skip-confirmation is not set
			if _, hasPRs := reposWithPRs[fork.NameWithOwner]; hasPRs && !skipConfirmation {
				color.New(color.FgRed, color.Bold).Print("❗ This fork has open PRs. Are you ABSOLUTELY sure you want to delete it? (yes/N): ")
				scanner.Scan()
				confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if confirm != "yes" {
					color.New(color.FgBlue).Printf("⏭️  Skipping %s...\n", fork.Name)
					continue
				}
			}
		}

		color.New(color.FgRed).Printf("🗑️  Deleting %s...\n", fork.Name)
		deleteCmd := exec.Command("gh", "repo", "delete", fork.Name, "--yes")
		if err := deleteCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", fork.Name, err)
		} else {
			color.New(color.FgGreen).Printf("✅ Successfully deleted %s.\n", fork.Name)
		}
	}
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("✨ Process complete!")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
