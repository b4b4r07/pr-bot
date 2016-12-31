package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2"
)

const STATE_OPEN string = "#67C63D"

var (
	repo = flag.String("repo", "", "Specify github.com repository name")
	user = flag.String("user", "", "Specify github.com user name")
)

func main() {
	flag.Parse()
	api := slack.New(os.Getenv("SLACK_TOKEN"))
	os.Exit(run(api))
}

func run(api *slack.Client) int {
	rtm := api.NewRTM()
	go rtm.ManageConnection()

	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				log.Print("Connected!")

			case *slack.MessageEvent:
				if ev.Text == "bot pr list" {
					issues, err := fetchIssuesFromGitHub(*user, *repo)
					if err != nil {
						log.Print(err)
						return 1
					}
					params := getPostMessageParameters(issues)
					_, _, err = api.PostMessage(ev.Channel, "", params)
					if err != nil {
						log.Print(err)
						return 1
					}
				}

			case *slack.InvalidAuthEvent:
				log.Print("Invalid credentials")
				return 1
			}
		}
	}
}

func getPostMessageParameters(issues []github.Issue) slack.PostMessageParameters {
	params := slack.PostMessageParameters{
		Markdown:  true,
		Username:  "pr-bot",
		IconEmoji: ":octocat:",
	}
	params.Attachments = []slack.Attachment{}
	for _, issue := range issues {
		labels := []string{}
		if issue.PullRequestLinks == nil {
			continue
		}
		for _, label := range issue.Labels {
			labels = append(labels, "`"+*label.Name+"`")
		}
		params.Attachments = append(params.Attachments, slack.Attachment{
			Fallback:   fmt.Sprintf("%d - %s", *issue.Number, *issue.Title),
			Title:      fmt.Sprintf("<%s|#%d> %s", *issue.HTMLURL, *issue.Number, *issue.Title),
			Text:       strings.Join(labels, ", "),
			MarkdownIn: []string{"title", "text", "fields", "fallback"},
			Color:      STATE_OPEN,
			AuthorIcon: *issue.User.AvatarURL,
			AuthorName: "@" + *issue.User.Login,
			AuthorLink: *issue.User.HTMLURL,
		})
	}
	return params
}

func fetchIssuesFromGitHub(user, repo string) ([]github.Issue, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	githubClient := github.NewClient(tc)

	opt := &github.IssueListByRepoOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var issues []github.Issue
	for {
		repos, resp, err := githubClient.Issues.ListByRepo(user, repo, opt)
		if err != nil {
			return []github.Issue{}, err
		}
		for _, v := range repos {
			issues = append(issues, *v)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	return issues, nil
}
