package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/cli/go-gh/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type Owner struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

type Repo struct {
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

func showSpinner(ctx context.Context, done chan bool) {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r") // Clear the spinner
			return
		case <-done:
			fmt.Print("\r") // Clear the spinner
			return
		case <-time.After(100 * time.Millisecond):
			fmt.Printf("\r%s Fetching forks", spinner[i])
			i = (i + 1) % len(spinner)
		}
	}
}

func getReposWithOpenPRs(ctx context.Context) (map[string][]PullRequestInfo, error) {
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

	stdout, stderr, err := gh.ExecContext(ctx, "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	if err != nil {
		return nil, fmt.Errorf("error fetching open PRs: %v\nOutput: %s", err, stderr.String())
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
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
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

func getForks(ctx context.Context) ([]Repo, error) {
	// GraphQL query to get all forks with pagination
	query := `
		query($after: String) {
			viewer {
				repositories(first: 100, after: $after, isFork: true, orderBy: {field: UPDATED_AT, direction: DESC}) {
					nodes {
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

		stdout, stderr, err := gh.ExecContext(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("error fetching forks: %v\nOutput: %s", err, stderr.String())
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
		if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
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

func getCommitComparison(ctx context.Context, fork Repo) (*CommitComparison, error) {
	if fork.Parent.NameWithOwner == "" || fork.Parent.DefaultBranchRef.Name == "" || fork.DefaultBranchRef.Name == "" {
		return nil, fmt.Errorf("missing required repository information")
	}

	// Use gh api to get the comparison between the fork and its parent
	stdout, stderr, err := gh.ExecContext(ctx,
		"api",
		fmt.Sprintf("repos/%s/compare/%s...%s:%s",
			fork.Parent.NameWithOwner,
			fork.Parent.DefaultBranchRef.Name,
			fork.Owner.Login,
			fork.DefaultBranchRef.Name,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error comparing repositories: %w\nOutput: %s", err, stderr.String())
	}

	var comparison CommitComparison
	if err := json.Unmarshal(stdout.Bytes(), &comparison); err != nil {
		return nil, fmt.Errorf("error parsing comparison response: %w", err)
	}

	return &comparison, nil
}

var rootCmd = &cobra.Command{
	Use:   "gh fork-cleanup",
	Short: "Clean up your GitHub forks",
	Long: `A CLI tool to help you clean up your GitHub forks.
It shows you all your forks, highlighting those that haven't been updated recently
and allows you to delete them if they don't have any open pull requests.`,
	RunE:          cleanupForks,
	SilenceUsage:  true, // Don't show usage on error
	SilenceErrors: true, // Disable cobra error handling. Errors are handled in the main function, we skip some of them
}

func init() {
	rootCmd.Flags().BoolP("skip-confirmation", "s", false, "Skip confirmation for forks with open pull requests")
	rootCmd.Flags().BoolP("force", "f", false, "It will automatically delete all forks. Be careful when using this option.")
}

func cleanupForks(cmd *cobra.Command, args []string) error {
	// retrieve the context set in the main function
	ctx := cmd.Context()

	// Start spinner
	done := make(chan bool)
	go showSpinner(ctx, done)

	// Get flags
	force, _ := cmd.Flags().GetBool("force")
	skipConfirmation, _ := cmd.Flags().GetBool("skip-confirmation")

	// Get all repos with open PRs
	color.New(color.FgBlue).Println("Fetching repositories with open pull requests...")
	reposWithPRs, err := getReposWithOpenPRs(ctx)
	if err != nil {
		return err
	}

	// Fetch all forks using GraphQL
	forks, err := getForks(ctx)
	if err != nil {
		return err
	}

	// Stop spinner
	done <- true

	if len(forks) == 0 {
		fmt.Println("No forked repositories found.")
		return nil
	}

	color.New(color.FgCyan, color.Bold).Printf("📦 Found %d forks\n", len(forks))
	scanner := newScanner(os.Stdin)
	for _, fork := range forks {
		fmt.Print("\n")
		color.New(color.FgGreen, color.Bold).Printf("📂 Repository: %s\n", fork.NameWithOwner)
		color.New(color.FgBlue).Printf("   🔄 Forked from: %s\n", fork.Parent.NameWithOwner)
		if fork.IsArchived {
			color.New(color.FgRed).Println("   📦 This repository is archived")
		}

		// Show commit comparison information
		if comparison, err := getCommitComparison(ctx, fork); err == nil {
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
			err := scanner.Read(ctx)
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			answer := strings.ToLower(strings.TrimSpace(scanner.Text()))

			if answer == "o" {
				repoURL := fmt.Sprintf("https://github.com/%s", fork.NameWithOwner)
				openCmd := exec.CommandContext(ctx, "xdg-open", repoURL)
				if err := openCmd.Run(); err != nil {
					// this is a non-fatal error, just print a message
					// no need to stop the program
					fmt.Fprintf(os.Stderr, "Error opening URL: %v\n", err)
				}
				// Ask again after opening the URL
				color.New(color.FgMagenta).Print("❔ Delete this repository? (y/n, default n): ")
				err := scanner.Read(ctx)
				if err != nil {
					return fmt.Errorf("error reading input: %v", err)
				}
				answer = strings.ToLower(strings.TrimSpace(scanner.Text()))
			}

			if answer != "y" {
				color.New(color.FgBlue).Printf("⏭️  Skipping %s...\n", fork.NameWithOwner)
				continue
			}

			// Double confirm if there are open PRs and skip-confirmation is not set
			if _, hasPRs := reposWithPRs[fork.NameWithOwner]; hasPRs && !skipConfirmation {
				color.New(color.FgRed, color.Bold).Print("❗ This fork has open PRs. Are you ABSOLUTELY sure you want to delete it? (yes/N): ")
				err := scanner.Read(ctx)
				if err != nil {
					return fmt.Errorf("error reading input: %v", err)
				}
				confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if confirm != "yes" {
					color.New(color.FgBlue).Printf("⏭️  Skipping %s...\n", fork.NameWithOwner)
					continue
				}
			}
		}

		color.New(color.FgRed).Printf("🗑️  Deleting %s...\n", fork.NameWithOwner)
		stdout, stderr, err := gh.ExecContext(ctx, "repo", "delete", fork.NameWithOwner, "--yes")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v %s\n", fork.NameWithOwner, err, stderr.String())
		} else {
			color.New(color.FgGreen).Printf("✅ Successfully deleted %s.\n", stdout.String())
		}
	}
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("✨ Process complete!")

	return nil
}

// run executes the root command and handles context cancellation.
//
// It returns an exit code based on the command execution result.
// If the command is canceled (e.g., by Ctrl+C), it returns 130.
// If an error occurs, it prints the error to stderr and returns 1.
// Otherwise, it returns 0 for a successful execution.
func run(ctx context.Context) int {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			// handle the CTRL+C case silently
			return 130 // classic exit code for a SIGINT (Ctrl+C) termination
		}

		fmt.Fprintln(os.Stderr, err)
		return 1 // return a non-zero exit code for any other error
	}

	return 0 // success
}

func main() {
	ctx := context.Background()
	os.Exit(run(ctx))
}
