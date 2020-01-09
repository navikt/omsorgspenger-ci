package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
	"gopkg.in/src-d/go-git.v4"
)

func main() {
	CheckArgs("<environment>\nWhere cwd is a repo and environment is prod | q \nThe head ref is matched against tags.")

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
	environment := os.Args[1]
	branch := head.Name().Short()

	CheckIfError(err)
	output, err := exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
	CheckIfError(err)
	shortHash := strings.TrimSpace(string(output))

	promptForAncestor(branch, r)

	commit, err := r.CommitObject(head.Hash())
	tag := fmt.Sprintf("%s-%s", commit.Author.When.Format("2006.01.02"), shortHash)

	repoName := url[index+1 : len(url)-4]

	if environment == "prod" {
		prompt := promptui.Prompt{
			Label:     "Deploy to prod?",
			IsConfirm: true,
		}

		_, err := prompt.Run()
		if err != nil {
			os.Exit(0)
			return
		}
		CheckIfError(err)

	}

	conf := readConfig()

	githubClient, ctx := getGithubClient(conf)

	promptConfirm(repoName, environment)

	repo, _, err := githubClient.Repositories.Get(ctx, "navikt", repoName)
	CheckIfError(err)

	env := ""

	if environment == "q" {
		env = "dev-sbs"
	} else {
		return
	}

	var payload = PayloadGithub{
		Ref:              shortHash,
		Environment:      env,
		Auto_merge:       false,
		Required_context: []string{},
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
	Ref              string   `json:"ref"`
	Environment      string   `json:"environment"`
	Auto_merge       bool     `json:"auto_merge"`
	Required_context []string `json:"required_context"`
	Payload          Payload  `json:"payload"`
}

type Payload struct {
	Triggered bool   `json:"triggered"`
	Image     string `json:"image"`
	Tag       string `json:"tag"`
}
