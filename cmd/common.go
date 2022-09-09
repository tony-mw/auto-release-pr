package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	http2 "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func (s PrConfig) SwitchBranch(r *git.Repository, wt *git.Worktree, branchRef plumbing.ReferenceName) {
	logger.Printf("switching to %s", branchRef.String())
	branchOpts := git.CheckoutOptions{
		Branch: branchRef,
	}
	err := wt.Checkout(&branchOpts)
	if err != nil {
		log.Fatal(err)
	}
	_, err = r.Head()
	if err != nil {
		log.Fatal(err)
	}
	return
}

func (v VersionFile) InMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	//Give me Billy.file
	logger.Println("Reading auth path...")
	tp, err := fs.Open(authoritativePath)
	if err != nil {
		log.Fatal(err)
	}
	//Create a new read buffer
	rd := bufio.NewReader(tp)
	dataYaml, err := ioutil.ReadAll(rd)
	if err != nil {
		log.Fatal(err)
	}
	logger.Println("Read complete")
	return dataYaml
}

func (a AppConfigFile) InMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	logger.Println("Reading staging path...")
	tp, err := fs.Open(stagingPath)
	if err != nil {
		log.Fatal(err)
	}
	//Create a new read buffer
	rd := bufio.NewReader(tp)
	dataYaml, err := ioutil.ReadAll(rd)
	if err != nil {
		log.Fatal(err)
	}
	logger.Println("Read complete")
	return dataYaml
}

func ReadFile(f File, authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	return f.InMemoryRead(authoritativePath, stagingPath, fs)
}

func (s PrConfig) SetLocalRepoSlug() string {

	var localRepoSlug string

	if s.ProdRepoSlug != "" {
		localRepoSlug = s.ProdRepoSlug
	} else {
		localRepoSlug = s.StagingRepoSlug
	}
	return localRepoSlug

}

func CleanWorkTree(wt *git.Worktree, fileStrings []string) error {
	ss1, err := wt.Status()
	if err != nil {
		return err
	}
	for k, v := range ss1 {
		logger.Println("Worktree status for: ", k, v.Extra, v.Worktree)
		for _, vv := range fileStrings {
			if strings.Contains(k, vv) {
				wt.Remove(k)
			}
		}
	}
	return nil
}

func (s PrConfig) IsStaging() bool {
	if s.ProdRepoSlug == "" {
		return true
	} else {
		return false
	}
}

func IncreaseFetchDepth(r *git.Repository, f git.FetchOptions, depth int) error {
	depth += 10
	f.Depth = depth
	err := r.Fetch(&f)
	if err != nil {
		if depth == 200 {
			return fmt.Errorf("Depth is at 200 - this is the limit: %s", err)
		}
		_ = IncreaseFetchDepth(r, f, depth)
	}
	return nil
}

func (s PrConfig) OpenPullRequest() {

	localRepoSlug := s.SetLocalRepoSlug()

	body := PullRequestPayload{
		FromRef: struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}{fmt.Sprintf("refs/heads/%s", s.SourceBranch), "BRANCH"},
		ToRef: struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}{"refs/heads/main", "BRANCH"},
		Title:       fmt.Sprintf("Candidate release to staging: %s", s.SourceBranch),
		Description: fmt.Sprintf("Candidate release to staging: %s", s.SourceBranch),
	}

	jsonBody, _ := json.Marshal(body)
	httpClient := &http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/projects/%s/repos/%s/pull-requests", bbBaseUrl, s.BBProject, localRepoSlug), bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", bitBucketCredentialString))
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode > 201 {
		err := fmt.Errorf("wrong status code when trying to create PR: %d", resp.StatusCode)
		logger.Println(localRepoSlug)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		logger.Println("Pull request was opened.")
	}

	return
}

func (s PrConfig) CheckoutBranch(exists bool) {

	//Create branch if it doesn't already exist
	localRepoSlug := s.SetLocalRepoSlug()

	if !exists {
		logger.Printf("trying to create branch: %s\n", s.SourceBranch)

		body := CreateBranchPayload{
			Message:    "Release Branch",
			Name:       s.SourceBranch,
			StartPoint: "main",
		}

		jsonBody, err := json.Marshal(body)
		httpClient := &http.Client{}
		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/projects/%s/repos/%s/branches", bbBaseUrl, s.BBProject, localRepoSlug), bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", bitBucketCredentialString))
		req.Header.Set("X-Atlassian-Token", "no-check")
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode > 201 {
			log.Fatalf("wrong status code when trying to create branch: %d", resp.StatusCode)
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
	}
	logger.Printf("trying to clone repo: %s\n", localRepoSlug)
	fs := memfs.New()
	//Clone the repo into memory
	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		//https://bitbucket.dentsplysirona.com/scm/atopoc/dpns-gitops-prod.git
		URL:   fmt.Sprintf("https://%s/scm/%s/%s.git", repoBaseUrl, s.BBProject, localRepoSlug),
		Auth:  &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
		Depth: 10,
		//ReferenceName: plumbing.ReferenceName(s.SourceBranch),
	})

	if err != nil {
		log.Fatal(err)
	}
	logger.Println("fetching...")
	f := git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*"},
		Auth:     &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
		Depth:    10,
	}
	err = r.Fetch(&f)
	if err != nil {
		logger.Println("Error fetching... Starting recursive function with increasing fetch depth...")
		err = IncreaseFetchDepth(r, f, f.Depth)
	}
	logger.Println("fetching done!")
	//Check out the working tree
	wt, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}
	if s.IsStaging() {

		var foundLocal bool

		//Set up my branch options so I can create or checkout the branch
		sb := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))

		s.SwitchBranch(r, wt, sb)

		refs, err := r.References()
		if err != nil {
			log.Fatal(err)
		}

		b := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))
		refs.ForEach(func(ref *plumbing.Reference) error {
			if ref.Name() == b {
				logger.Printf("reference exists locally:\n%s\n", ref)
				foundLocal = true
			}
			return nil
		})
		if !foundLocal {
			logger.Printf("reference %s does not exist locally\n", b)
		}

		s.UpdateVersionFiles(r, wt, fs, nil)
		s.CommitAndPush(r, wt)
		return
	} else {
		logger.Println("checkout staging repo and branch and copy everything from staging to prod.")
		fs1 := memfs.New()
		//Clone the repo into memory
		r1, err := git.Clone(memory.NewStorage(), fs1, &git.CloneOptions{
			//https://bitbucket.dentsplysirona.com/scm/atopoc/dpns-gitops-prod.git
			URL:   fmt.Sprintf("https://%s/scm/%s/%s.git", repoBaseUrl, s.BBProject, s.StagingRepoSlug),
			Auth:  &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
			Depth: 10,
		})
		if err != nil {
			log.Fatal(err)
		}
		err = r1.Fetch(&f)
		if err != nil {
			logger.Println("error fetching from second repo (staging)... Starting recursive function with increasing fetch depth...")
			err = IncreaseFetchDepth(r1, f, f.Depth)
		}
		logger.Println("Fetching done!")
		s.UpdateVersionFiles(r, wt, fs, fs1)
		s.CommitAndPush(r, wt)
	}
}

func (s PrConfig) UpdateManifests(r *git.Repository, wt *git.Worktree, fs billy.Filesystem, fs1 billy.Filesystem, wg *sync.WaitGroup, service string) {
	defer wg.Done()
	var authoritativeManifestPath string
	var destManifestPath string
	var manifestFileInfo []os.FileInfo
	var destinationManifestFileInfo []os.FileInfo

	var err error

	if s.IsStaging() {
		authoritativeManifestPath = fmt.Sprintf("%s/services/%s/manifests/base/main", s.Product, service)
		destManifestPath = fmt.Sprintf("%s/services/%s/manifests/base/staging", s.Product, service)
		manifestFileInfo, err = fs.ReadDir(authoritativeManifestPath)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		authoritativeManifestPath = fmt.Sprintf("%s/services/%s/manifests/base/staging", s.Product, service)
		destManifestPath = fmt.Sprintf("%s/services/%s/manifests/base", s.Product, service)
		manifestFileInfo, err = fs1.ReadDir(authoritativeManifestPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	destinationManifestFileInfo, err = fs.ReadDir(destManifestPath)
	if err != nil {
		log.Fatal(err)
	}

	//Delete dest files first
	for _, manifestFile := range destinationManifestFileInfo {
		destPath := filepath.Join(destManifestPath, manifestFile.Name())
		err := fs.Remove(destPath)
		if err != nil {
			log.Fatal(err)
		}
		myAdd, err := wt.Add(destPath)
		if err != nil {
			log.Fatal(err)
		}
		logger.Println("Deleted: ", myAdd.String())
	}

	for _, manifestFile := range manifestFileInfo {
		//TODO: Need to check if a file was deleted in source and then delete it in destination
		sourcePath := filepath.Join(authoritativeManifestPath, manifestFile.Name())
		destPath := filepath.Join(destManifestPath, manifestFile.Name())
		outFile, err := fs.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatal(err)
		}
		defer outFile.Close()

		var sourceFs billy.Filesystem

		if s.IsStaging() {
			sourceFs = fs
		} else {
			sourceFs = fs1
		}

		inFile, err := sourceFs.Open(sourcePath)
		logger.Println("In file is: ", inFile.Name())
		logger.Println("Out file is: ", outFile.Name())
		if err != nil {
			log.Fatal(err)
		}
		defer inFile.Close()

		_, err = io.Copy(outFile, inFile)
		if err != nil {
			log.Fatal(err)
		}
		_, err = wt.Add(destPath)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (s PrConfig) UpdateVersionFiles(r *git.Repository, wt *git.Worktree, fs billy.Filesystem, fs1 billy.Filesystem) {

	var wg sync.WaitGroup

	for _, v := range s.Services {
		wg.Add(1)

		var dPath string
		var myVersionData []byte

		versionFile := VersionFile{}
		appConfig := AppConfigFile{}

		//Set up my branch options so I can create or checkout the branch
		//Switch to main to get updated test semver.yaml
		sbt := plumbing.ReferenceName("refs/heads/main")
		s.SwitchBranch(r, wt, sbt)

		authoritativePath := fmt.Sprintf("%s/services/%s/images/latest/.semver.yaml", s.Product, v)
		logger.Println("\n", authoritativePath)

		//Probably a more optimal way than if else this stuff
		//Load the source file
		if s.IsStaging() {
			dPath = fmt.Sprintf("%s/.argocd/staging/%s/config.yaml", s.Product, v)
			myVersionData = ReadFile(versionFile, authoritativePath, dPath, fs)
		} else {
			dPath = fmt.Sprintf("%s/.argocd/production/r2/%s/config.yaml", s.Product, v)
			myVersionData = ReadFile(versionFile, authoritativePath, dPath, fs1)
		}

		logger.Println("read version data.")
		//Clean Up Worktree
		err := CleanWorkTree(wt, []string{".vscode", ".idea"})
		if err != nil {
			log.Fatal(err)
		}
		//Switch back to staging to update file
		logger.Println("switching back to: ", s.SourceBranch)
		sbs := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))
		s.SwitchBranch(r, wt, sbs)

		//Spin off into another goRoutine here to update Manifest files
		//Need to figure this out for prod
		if s.IsStaging() {
			go s.UpdateManifests(r, wt, fs, nil, &wg, v)
		} else {
			go s.UpdateManifests(r, wt, fs, fs1, &wg, v)
		}
		var myAppConfigData []byte

		myAppConfigData = ReadFile(appConfig, authoritativePath, dPath, fs)

		err = yaml.Unmarshal(myVersionData, &versionFile)
		if err != nil {
			log.Fatal(err)
		}
		logger.Println("the version data: ", versionFile)

		err = yaml.Unmarshal(myAppConfigData, &appConfig)
		if err != nil {
			log.Fatal(err)
		}
		logger.Println("the app Config data: ", appConfig)

		//Update Version Value for Staging!!!!
		appConfig.App.ImageTag = fmt.Sprintf("%s-%s", versionFile.Release, versionFile.CommitHash)

		//Rewrite Yaml
		cfg, err := fs.OpenFile(dPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatal(err)
		}
		appConfigNewYaml, err := yaml.Marshal(appConfig)
		if err != nil {
			log.Fatal(err)
		}
		err = util.WriteFile(fs, cfg.Name(), appConfigNewYaml, 0644)

		if err != nil {
			log.Fatal(err)
		}

		content, err := ioutil.ReadAll(cfg)
		if err != nil {
			log.Fatal(err)
		}
		logger.Println("New content is: ", string(content))

		wg.Wait()

		logger.Println("Getting worktree status for service: ", v)
		ss, err := wt.Status()
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range ss {
			logger.Println("Worktree status for: ", k, v.Extra, v.Worktree)
		}
		//More changes for prod/staging
		var myAdd plumbing.Hash

		if s.IsStaging() {
			myAdd, err = wt.Add(fmt.Sprintf("%s/.argocd/staging/%s/config.yaml", s.Product, v))
		} else {
			myAdd, err = wt.Add(fmt.Sprintf("%s/.argocd/production/r2/%s/config.yaml", s.Product, v))
		}
		if err != nil {
			log.Fatal(err)
		}
		logger.Println(myAdd.String())

		for k, v := range ss {
			logger.Println("Worktree status for: ", k, v.Extra, v.Worktree)
		}

		_, err = wt.Commit("Auto commit version update for release", &git.CommitOptions{})
		if err != nil {
			log.Fatal("An error occurred committing", err)
		}

	}

	logger.Println("Version files updated")

	return
}

func (s PrConfig) CommitAndPush(r *git.Repository, wt *git.Worktree) {

	pushOptions := git.PushOptions{
		RemoteName: "origin",
		//RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", s.SourceBranch, s.SourceBranch))},
		Auth: &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
	}
	err := r.Push(&pushOptions)
	if err != nil {
		log.Fatal(err)
	}
	logger.Println("Commit and push complete")
	return
}

func (s PrConfig) CheckBranchExists() (bool, error) {

	logger.Println("checking for branch")
	client := &http.Client{}

	localRepoSlug := s.SetLocalRepoSlug()

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/projects/%s/repos/%s/branches?filterText=%s", bbBaseUrl, s.BBProject, localRepoSlug, s.SourceBranch), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", bitBucketCredentialString))
	response, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	logger.Println(response.StatusCode, "\n", string(data))
	if len(string(data)) > 61 {
		return true, nil
	} else if len(string(data)) == 61 {
		return false, nil
	} else {
		return false, fmt.Errorf("seems like the length of the response didn't match the predicted outcomes, it was: %d", len(string(data)))
	}
}

func (s PrConfig) CheckPullRequestExists() (bool, error) {
	logger.Println("Checking for pull request")
	httpClient := &http.Client{}

	localRepoSlug := s.SetLocalRepoSlug()

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/projects/%s/repos/%s/pull-requests?filterText=%s", bbBaseUrl, s.BBProject, localRepoSlug, s.SourceBranch), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", bitBucketCredentialString))
	response, err := httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	if len(string(data)) > 61 {
		return true, nil
	} else if len(string(data)) == 61 {
		return false, nil
	} else {
		return false, fmt.Errorf("seems like the length of the response didn't match the predicted outcomes, it was: %d", len(string(data)))
	}
}

func PrepRelease(c Config) {
	branchExists, err := c.CheckBranchExists()
	if err != nil {
		log.Fatal(err)
	}
	if branchExists {
		c.CheckoutBranch(true)
	} else {
		c.CheckoutBranch(false)
	}

	prExists, err := c.CheckPullRequestExists()
	if err != nil {
		log.Fatal(err)
	}
	if prExists {
		return
	} else {
		logger.Println("opening pull request...")
		c.OpenPullRequest()
	}
}
