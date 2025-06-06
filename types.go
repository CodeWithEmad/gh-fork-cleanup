package main

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
