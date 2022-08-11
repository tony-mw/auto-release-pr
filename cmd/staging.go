/*
Copyright © 2022 Tony Prestifilippo
*/

package cmd

import (
	"bitbucket.dentsplysirona.com/atopoc/auto-release-pr/utils"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/cobra"
	http2 "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//https://bitbucket.dentsplysirona.com/scm/atopoc/cirrus-poc-gitops.git
const (
	bbBaseUrl   = "bitbucket.dentsplysirona.com/rest/api/1.0"
	repoBaseUrl = "bitbucket.dentsplysirona.com/scm"
	//username    = "USERNAME"
	//password    = "PASSWORD"
	username = "TEMPUSER"
	password = "BBTOKEN"
)

var bitBucketCredentialString string = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv(username), os.Getenv(password))))

var debugOn utils.Logger = utils.Logger{Debug: false}
var fatalError utils.Error = utils.Error{Fatal: true}

type File interface {
	inMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte
}

type StagingConfig struct {
	RepoSlug     string
	BBProject    string
	SourceBranch string
	Product      string
	Services     []string
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

type AppConfig struct {
	App struct {
		Source    string `yaml:"source"`
		Path      string `yaml:"path"`
		Revision  string `yaml:"revision"`
		ImageName string `yaml:"image_name"`
		ImageTag  string `yaml:"image_tag"`
	} `yaml:"app"`
}

func (s StagingConfig) PrepRelease() {
	branchExists, err := s.CheckBranchExists()
	if err != nil {
		log.Fatal(err)
	}
	if branchExists {
		s.checkoutBranch(true)
	} else {
		s.checkoutBranch(false)
	}

	prExists, err := s.CheckPullRequestExists()
	if err != nil {
		log.Fatal(err)
	}
	if prExists {
		return
	} else {
		s.OpenPullRequest()
	}
}

func (s StagingConfig) OpenPullRequest() {

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
	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/projects/%s/repos/%s/pull-requests", bbBaseUrl, s.BBProject, s.RepoSlug), bytes.NewBuffer(jsonBody))
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
	if resp.StatusCode != 200 {
		fmt.Errorf("wrong status code when trying to create branch: %d", resp.StatusCode)
	}

	return
}

func (s StagingConfig) CheckBranchExists() (bool, error) {
	fmt.Println("Checking for branch")
	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/projects/%s/repos/%s/branches?filterText=%s", bbBaseUrl, s.BBProject, s.RepoSlug, s.SourceBranch), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", bitBucketCredentialString))
	response, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.StatusCode, "\n", string(data))
	if len(string(data)) > 61 {
		return true, nil
	} else if len(string(data)) == 61 {
		return false, nil
	} else {
		return false, fmt.Errorf("seems like the length of the response didn't match the predicted outcomes, it was: %d", len(string(data)))
	}
}

func (s StagingConfig) CheckPullRequestExists() (bool, error) {
	fmt.Println("Checking for pull request")
	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/projects/%s/repos/%s/pull-requests?filterText=%s", bbBaseUrl, s.BBProject, s.RepoSlug, s.SourceBranch), nil)
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

func increaseFetchDepth(r *git.Repository, f git.FetchOptions, depth int) error {
	depth += 10
	f.Depth = depth
	err := r.Fetch(&f)
	if err != nil {
		if depth == 200 {
			return fmt.Errorf("Depth is at 200 - this is the limit: %s", err)
		}
		_ = increaseFetchDepth(r, f, depth)
	}
	return nil
}

func (s StagingConfig) checkoutBranch(exists bool) {
	//Create branch if it doesn't already exist
	if !exists {
		fmt.Println("Trying to create branch...")
		body := CreateBranchPayload{
			Message:    "Release Branch",
			Name:       s.SourceBranch,
			StartPoint: "main",
		}
		jsonBody, _ := json.Marshal(body)

		httpClient := &http.Client{}

		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/projects/%s/repos/%s/branches", bbBaseUrl, s.BBProject, s.RepoSlug), bytes.NewBuffer(jsonBody))
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
		if resp.StatusCode != 200 {
			log.Fatalf("wrong status code when trying to create branch: %d", resp.StatusCode)
		}
		fmt.Println(resp.StatusCode)
		r, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(r))
	}
	fmt.Println("Trying to clone repo...")
	fs := memfs.New()
	//Clone the repo into memory
	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		//https://bitbucket.dentsplysirona.com/scm/atopoc/dpns-gitops-prod.git
		URL:   fmt.Sprintf("https://%s/scm/%s/%s.git", repoBaseUrl, s.BBProject, s.RepoSlug),
		Auth:  &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
		Depth: 10,
		//ReferenceName: plumbing.ReferenceName(s.SourceBranch),
	})

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Fetching...")
	f := git.FetchOptions{
		//RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
		//RefSpecs: []config.RefSpec{"refs/*:refs/*",},
		//RefSpecs: []config.RefSpec{"refs/*:refs/*"},
		RefSpecs: []config.RefSpec{"refs/*:refs/*"},
		Auth:     &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
		Depth:    10,
	}
	err = r.Fetch(&f)
	if err != nil {
		fmt.Println("Error fetching... Starting recursive function with increasing fetch depth...")
		err = increaseFetchDepth(r, f, f.Depth)
	}
	fmt.Println("Fetching done!")
	//Check out the working tree
	wt, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}

	//Set up my branch options so I can create or checkout the branch
	sb := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))
	s.switchBranch(r, wt, sb)

	refs, err := r.References()

	var foundLocal bool
	b := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name() == b {
			fmt.Printf("reference exists locally:\n%s\n", ref)
			foundLocal = true
		}
		return nil
	})
	if !foundLocal {
		fmt.Printf("reference %s does not exist locally\n", b)
	}

	s.updateVersionFiles(r, wt, fs)
	s.commitAndPush(r, wt)
	return
}

func (s StagingConfig) switchBranch(r *git.Repository, wt *git.Worktree, branchRef plumbing.ReferenceName) {
	fmt.Println("Switching to ", branchRef)
	branchOpts := git.CheckoutOptions{
		Branch: branchRef,
	}
	err := wt.Checkout(&branchOpts)
	if err != nil {
		log.Fatal(err)
	}
	cb, err := r.Head()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cb.Name())
	return
}

func (v VersionFile) inMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	//Give Billy.file
	fmt.Println("Reading auth path...")
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
	return dataYaml
}

func (a AppConfig) inMemoryRead(authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	fmt.Println("Reading staging path...")
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
	fmt.Println("Read complete")
	return dataYaml
}

func readFile(f File, authoritativePath string, stagingPath string, fs billy.Filesystem) []byte {
	return f.inMemoryRead(authoritativePath, stagingPath, fs)
}

func (s StagingConfig) UpdateManifests(r *git.Repository, wt *git.Worktree, fs billy.Filesystem, wg *sync.WaitGroup, service string) {
	defer wg.Done()
	authoritativeManifestPath := fmt.Sprintf("%s/kustomize/%s/base/main", s.Product, service)
	stagingManifestPath := fmt.Sprintf("%s/kustomize/%s/base/staging", s.Product, service)
	manifestFileInfo, err := fs.ReadDir(authoritativeManifestPath)
	if err != nil {
		log.Fatal(err)
	}
	destinationManifestFileInfo, err := fs.ReadDir(stagingManifestPath)
	if err != nil {
		log.Fatal(err)
	}
	//Delete dest files first
	for _, manifestFile := range destinationManifestFileInfo {
		fs.Remove(manifestFile.Name())
	}

	for _, manifestFile := range manifestFileInfo {
		//TODO: Need to check if a file was deleted in source and then delete it in destination
		sourcePath := filepath.Join(authoritativeManifestPath, manifestFile.Name())
		destPath := filepath.Join(stagingManifestPath, manifestFile.Name())
		outFile, err := fs.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatal(err)
		}
		defer outFile.Close()
		inFile, err := fs.Open(sourcePath)
		if err != nil {
			log.Fatal(err)
		}
		defer inFile.Close()

		_, err = io.Copy(outFile, inFile)
		if err != nil {
			log.Fatal(err)
		}
		myAdd, err := wt.Add(destPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf(myAdd.String())
	}
}

func (s StagingConfig) updateVersionFiles(r *git.Repository, wt *git.Worktree, fs billy.Filesystem) {

	var wg sync.WaitGroup

	for _, v := range s.Services {
		wg.Add(1)

		fmt.Println("\n", strings.TrimSpace(v))

		versionFile := VersionFile{}
		appConfig := AppConfig{}

		//Set up my branch options so I can create or checkout the branch
		//Switch to main to get updated test semver.yaml
		sbt := plumbing.ReferenceName("refs/heads/main")
		s.switchBranch(r, wt, sbt)


		authoritativePath := fmt.Sprintf("%s/images/latest/%s/.semver.yaml", s.Product, v)
		fmt.Println("\n", authoritativePath)
		stagingPath := fmt.Sprintf("%s/.argocd/staging/%s/config.yaml", s.Product, v)
		////Give me billy file
		//tp, err := fs.Open(authoritativePath)
		//if err != nil {
		//	log.Fatal(err)
		//}
		////Create a new read buffer
		//rd := bufio.NewReader(tp)

		myVersionData := readFile(versionFile, authoritativePath, stagingPath, fs)

		//Switch back to staging to update file
		sbs := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", s.SourceBranch))
		s.switchBranch(r, wt, sbs)

		//Spin off into another goRoutine here to update Manifest files
		go s.UpdateManifests(r, wt, fs, &wg, v)

		myAppConfigData := readFile(appConfig, authoritativePath, stagingPath, fs)

		err := yaml.Unmarshal(myVersionData, &versionFile)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("The version data dog: ", versionFile)

		err = yaml.Unmarshal(myAppConfigData, &appConfig)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("The app Config data dog: ", appConfig)

		//Update Version Value for Staging!!!!
		appConfig.App.ImageTag = fmt.Sprintf("%s-%s", versionFile.Release, versionFile.CommitHash)

		//Rewrite Yaml
		cfg, err := fs.OpenFile(stagingPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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
		fmt.Println("New content is: ", string(content))

		wg.Wait()

		fmt.Println("Getting worktree status for service: ", v)
		ss, err := wt.Status()
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range ss {
			fmt.Println("Worktree status for: ", k, v.Extra, v.Worktree)
		}
		myAdd, err := wt.Add(fmt.Sprintf("%s/.argocd/staging/%s/config.yaml", s.Product, v))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(myAdd.String())

		for k, v := range ss {
			fmt.Println("Worktree status for: ", k, v.Extra, v.Worktree)
		}

		_, err = wt.Commit("Auto commit version update for release", &git.CommitOptions{})
		if err != nil {
			log.Fatal("An error occurred committing", err)
		}

	}

	fmt.Println("Version files updated")
	return
}

func (s StagingConfig) commitAndPush(r *git.Repository, wt *git.Worktree) {

	pushOptions := git.PushOptions{
		RemoteName: "origin",
		//RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", s.SourceBranch, s.SourceBranch))},
		Auth: &http2.BasicAuth{Username: os.Getenv(username), Password: os.Getenv(password)},
	}
	err := r.Push(&pushOptions)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Commit and push complete")
	return
}

// stagingCmd represents the staging command
var stagingCmd = &cobra.Command{
	Use:   "staging",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("staging called")
		repoSlug, _ := cmd.Flags().GetString("repo-slug")
		bbProject, _ := cmd.Flags().GetString("bitbucket-project")
		sourceBranch, _ := cmd.Flags().GetString("source-branch")
		product, _ := cmd.Flags().GetString("product")
		services, _ := cmd.Flags().GetStringSlice("services")

		myStagingConfig := StagingConfig{
			RepoSlug:     repoSlug,
			BBProject:    bbProject,
			SourceBranch: sourceBranch,
			Product:      product,
			Services:     services,
		}

		myStagingConfig.PrepRelease()
	},
}

func init() {
	rootCmd.AddCommand(stagingCmd)
	// Here you will define your flags and configuration settings.
	stagingCmd.PersistentFlags().String("repo-slug", "", "The repository slug")
	stagingCmd.PersistentFlags().String("bitbucket-project", "", "The repository bitbucket project")
	stagingCmd.PersistentFlags().String("source-branch", "", "The branch to create")
	stagingCmd.PersistentFlags().String("product", "", "The product which will also be the top level directory of the repo")
	stagingCmd.PersistentFlags().StringSlice("services", []string{""}, "A list of the services that will be deployed to staging")
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stagingCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stagingCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
