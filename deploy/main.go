package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/manifoldco/promptui"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func main() {

	environment, shortHash, tag, repoName, branchName := getRepoInfo()

	CheckArgs("<environment>\nWhere cwd is a repo and environment is prod | dev-sbs or dev-fss \nThe head ref is matched against tags.")

	deploy(environment, branchName, repoName, shortHash, tag)
}

// reaches returns true if commit, c, can be reached from commit, start. Results are memoized in memo.
func reaches(r *git.Repository, start, c plumbing.Hash, memo map[plumbing.Hash]bool) (bool, error) {
	if v, ok := memo[start]; ok {
		return v, nil
	}
	if start == c {
		memo[start] = true
		return true, nil
	}
	co, err := r.CommitObject(start)
	if err != nil {
		return false, err
	}
	for _, p := range co.ParentHashes {
		v, err := reaches(r, p, c, memo)
		if err != nil {
			return false, err
		}
		if v {
			memo[start] = true
			return true, nil
		}
	}
	memo[start] = false
	return false, nil
}
func deploy(environment string, branchName string, repoName string, shortHash string, tag string) {
	promtGuardProd(environment, branchName)
	conf := readConfig()
	githubClient, ctx := getGithubClient(conf)
	promptConfirm(repoName, environment)
	repo, _, err := githubClient.Repositories.Get(ctx, "navikt", repoName)
	CheckIfError(err)
	env := ""
	if environment == "dev-sbs" {
		env = "dev-sbs"
	} else if environment == "dev-fss" {
		env = "dev-fss"
	} else {
		return
	}
	var payload = PayloadGithub{
		Ref:               shortHash,
		Environment:       env,
		Auto_merge:        false,
		Required_contexts: []string{},
		Payload: Payload{
			Triggered: true,
			Image:     repoName,
			Tag:       tag,
		},
	}
	bytes, err := json.Marshal(payload)
	log.Println(string(bytes))
	CheckIfError(err)
	reader := strings.NewReader(string(bytes))
	req, err := http.NewRequest("POST", repo.GetDeploymentsURL(), reader)
	CheckIfError(err)
	req.Header.Set("Authorization", "token "+conf.Githubtoken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Media-Type", "application/vnd.github.ant-man-preview+json")
	resp, err := http.DefaultClient.Do(req)
	CheckIfError(err)
	body, err := ioutil.ReadAll(resp.Body)
	CheckIfError(err)
	Info("Check build status:" + string(body))
}

func promtGuardProd(environment string, branchName string) {

	if environment == "prod" {
		if branchName != "master" {
			Warning("Can not deploy a non master branch to prod")
			os.Exit(0)
		}
		prompt := promptui.Prompt{
			Label:     "Deploy to prod?",
			IsConfirm: true,
		}

		_, err := prompt.Run()
		if err != nil {
			os.Exit(0)
		}
		CheckIfError(err)
	}
}

func getRepoInfo() (environment, shorthash, tag, reponame, branchName string) {
	r, err := git.PlainOpen(".")
	CheckIfError(err)
	_ = r.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
	head, err := r.Head()
	CheckIfError(err)
	config, err := r.Config()
	CheckIfError(err)
	url := config.Remotes["origin"].URLs[0]
	index := strings.LastIndex(url, "/")
	if len(os.Args) > 1 {
		environment = os.Args[1]
	}
	branchName = head.Name().Short()
	CheckIfError(err)

	output, err := exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
	CheckIfError(err)
	shortHash := strings.TrimSpace(string(output))

	promptForAncestor(branchName, r)
	commit, err := r.CommitObject(head.Hash())
	tag = fmt.Sprintf("%s-%s", commit.Author.When.Format("2006.01.02"), shortHash)

	repoName := url[index+1 : len(url)-4]
	CheckIfError(err)

	return environment, shortHash, tag, repoName, branchName
}

func promptForAncestor(branch string, r *git.Repository) {
	revHash, err := r.ResolveRevision(plumbing.Revision("origin/" + branch))
	CheckIfError(err)
	revCommit, err := r.CommitObject(*revHash)

	CheckIfError(err)

	headRef, err := r.Head()
	CheckIfError(err)
	headCommit, err := r.CommitObject(headRef.Hash())
	CheckIfError(err)

	isAncestor, err := revCommit.IsAncestor(headCommit)
	CheckIfError(err)
	if !isAncestor {
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Head is not updated, are you sure you want to deploy"),
			IsConfirm: true,
		}

		_, err := prompt.Run()
		if err != nil {
			os.Exit(0)
		}
		CheckIfError(err)
	}
}

func promptConfirm(tagName string, environment string) {
	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Deploy %s to %s", tagName, environment),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		os.Exit(0)
	}
	CheckIfError(err)
}

func getGithubClient(conf Config) (*github.Client, context.Context) {
	githubToken := conf.Githubtoken
	if len(githubToken) == 0 {
		githubToken = promtForGithubToken(conf)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc), ctx
}

func promtForGithubToken(config Config) string {
	validate := func(input string) error {
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Github token",
		Validate: validate,
	}
	result, err := prompt.Run()
	CheckIfError(err)

	config.Githubtoken = result
	confb, e := json.Marshal(config)
	if e != nil {
		log.Fatal(e)
	}
	homeDir, e := os.UserHomeDir()

	CheckIfError(e)
	e = ioutil.WriteFile(homeDir+"/.deploy.json", confb, 0666)
	CheckIfError(e)
	return result
}

func readConfig() Config {
	homeDir, err := os.UserHomeDir()
	CheckIfError(err)

	var config = Config{}
	bytes, err := ioutil.ReadFile(homeDir + "/.deploy.json")
	if err != nil {
		return config
	}

	err = json.Unmarshal(bytes, &config)
	CheckIfError(err)
	return config
}

type Config struct {
	Githubtoken string
}

type PayloadGithub struct {
	Ref               string   `json:"ref"`
	Environment       string   `json:"environment"`
	Auto_merge        bool     `json:"auto_merge"`
	Required_contexts []string `json:"required_contexts"`
	Payload           Payload  `json:"payload"`
}

type Payload struct {
	Triggered bool   `json:"triggered"`
	Image     string `json:"image"`
	Tag       string `json:"tag"`
}
