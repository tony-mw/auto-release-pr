package cmd

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"sync"
)

type File interface {
	InMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte
}

type Config interface {
	OpenPullRequest()
	CheckBranchExists() (bool, error)
	CheckPullRequestExists() (bool, error)
	CheckoutBranch(bool)
	SwitchBranch(*git.Repository, *git.Worktree, plumbing.ReferenceName)
	UpdateManifests(*git.Repository, *git.Worktree, billy.Filesystem, billy.Filesystem, *sync.WaitGroup, string)
	UpdateVersionFiles(*git.Repository, *git.Worktree, billy.Filesystem, billy.Filesystem)
	CommitAndPush(*git.Repository, *git.Worktree)
}

type PrConfig struct {
	StagingRepoSlug string
	ProdRepoSlug    string
	BBProject       string
	SourceBranch    string
	Product         string
	Services        []string
}

type CreateBranchPayload struct {
	Message    string `json:"message"`
	Name       string `json:"name"`
	StartPoint string `json:"startPoint"`
}

type PullRequestPayload struct {
	FromRef struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"fromRef"`
	ToRef struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"toRef"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type VersionFile struct {
	Alpha      int    `yaml:"alpha"`
	Beta       int    `yaml:"beta"`
	CommitHash string `yaml:"commit-hash"`
	Rc         int    `yaml:"rc"`
	Release    string `yaml:"release"`
}

type AppConfigFile struct {
	App struct {
		Source    string `yaml:"source"`
		Path      string `yaml:"path"`
		Revision  string `yaml:"revision"`
		ImageName string `yaml:"image_name"`
		ImageTag  string `yaml:"image_tag"`
	} `yaml:"app"`
}
