package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	// "strings"

	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
	"golang.org/x/oauth2"
)

const STATE_OPEN string = "#67C63D"

type Object struct {
	PullRequest github.PullRequest
	Statuses    []*github.RepoStatus
}

var Params slack.PostMessageParameters = slack.PostMessageParameters{
	Markdown:  true,
	Username:  "pr-bot",
	IconEmoji: ":octocat:",
}

var pattern *regexp.Regexp = regexp.MustCompile(`^bot\s+pr\s+(\w+)`)

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
				pat := pattern.FindStringSubmatch(ev.Text)
				if len(pat) > 1 {
					switch pat[1] {
					case "help":
						p := Params
						attachment := slack.Attachment{
							Fallback: "",
							Title:    "Usage:",
							Fields: []slack.AttachmentField{
								slack.AttachmentField{
									Title: ":small_red_triangle_down: pr list",
									Value: "List all opened P-Rs",
								},
							},
						}
						p.Attachments = []slack.Attachment{attachment}
						_, _, err := api.PostMessage(ev.Channel, "", p)
						if err != nil {
							log.Print(err)
							return 1
						}
					case "list":
						// issues, err := fetchIssuesFromGitHub(*user, *repo)
						// if err != nil {
						// 	log.Print(err)
						// 	return 1
						// }
						// p := getPostMessageParameters(issues)
						// _, _, err = api.PostMessage(ev.Channel, "", p)
						// if err != nil {
						// 	log.Print(err)
						// 	return 1
						// }
						prs, err := fetchPullReq(*user, *repo)
						if err != nil {
							log.Print(err)
							return 1
						}
						p := getPostMessageParameters(prs)
						_, _, err = api.PostMessage(ev.Channel, "", p)
						if err != nil {
							log.Print(err)
							return 1
						}
					default:
						p := Params
						attachment := slack.Attachment{
							Title: "Error",
							Text:  fmt.Sprintf("%s: no such command", pat[1]),
							Color: "danger",
						}
						p.Attachments = []slack.Attachment{attachment}
						_, _, err := api.PostMessage(ev.Channel, "", p)
						if err != nil {
							log.Print(err)
							return 1
						}
					}
				}

			case *slack.InvalidAuthEvent:
				log.Print("Invalid credentials")
				return 1
			}
		}
	}
}

// func getPostMessageParameters(prs []github.PullRequest) slack.PostMessageParameters {
func getPostMessageParameters(prs []Object) slack.PostMessageParameters {
	p := Params
	p.Attachments = []slack.Attachment{}
	for _, pr := range prs {
		// labels := []string{}
		// for _, label := range issue.Labels {
		// 	labels = append(labels, "`"+*label.Name+"`")
		// }
		// log.Print(pr.Head.SHA)
		// for _, state := range pr.Statuses {
		// 	// log.Printf("%#v\n", *state.ID)
		// 	log.Print(*state.ID, *state.URL, *state.State, *state.Description)
		// }
		text := ""
		log.Print(pr.Statuses)
		if len(pr.Statuses) > 0 {
			text = *pr.Statuses[0].State
		}
		p.Attachments = append(p.Attachments, slack.Attachment{
			Fallback:   fmt.Sprintf("%d - %s", *pr.PullRequest.Number, *pr.PullRequest.Title),
			Title:      fmt.Sprintf("<%s|#%d> %s", *pr.PullRequest.HTMLURL, *pr.PullRequest.Number, *pr.PullRequest.Title),
			Text:       text,
			MarkdownIn: []string{"title", "text", "fields", "fallback"},
			Color:      STATE_OPEN,
			AuthorIcon: *pr.PullRequest.User.AvatarURL,
			AuthorName: "@" + *pr.PullRequest.User.Login,
			AuthorLink: *pr.PullRequest.User.HTMLURL,
		})
	}
	return p
}

// func getPostMessageParameters(issues []github.Issue) slack.PostMessageParameters {
// 	p := Params
// 	p.Attachments = []slack.Attachment{}
// 	for _, issue := range issues {
// 		labels := []string{}
// 		if issue.PullRequestLinks == nil {
// 			continue
// 		}
// 		for _, label := range issue.Labels {
// 			labels = append(labels, "`"+*label.Name+"`")
// 		}
// 		p.Attachments = append(p.Attachments, slack.Attachment{
// 			Fallback:   fmt.Sprintf("%d - %s", *issue.Number, *issue.Title),
// 			Title:      fmt.Sprintf("<%s|#%d> %s", *issue.HTMLURL, *issue.Number, *issue.Title),
// 			Text:       strings.Join(labels, ", "),
// 			MarkdownIn: []string{"title", "text", "fields", "fallback"},
// 			Color:      STATE_OPEN,
// 			AuthorIcon: *issue.User.AvatarURL,
// 			AuthorName: "@" + *issue.User.Login,
// 			AuthorLink: *issue.User.HTMLURL,
// 		})
// 	}
// 	return p
// }

func fetchPullReq(user, repo string) ([]Object, error) {
	if user == "" || repo == "" {
		// return []github.PullRequest{}, errors.New("user/repo invalid format")
		return []Object{}, errors.New("user/repo invalid format")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	githubClient := github.NewClient(tc)

	opt := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	// var prs []github.PullRequest
	var obj []Object
	for {
		repos, resp, err := githubClient.PullRequests.List(user, repo, opt)
		if err != nil {
			// return []github.PullRequest{}, err
			return []Object{}, err
		}
		for _, v := range repos {
			// prs = append(prs, *v)
			repoStatuses, _, _ := githubClient.Repositories.ListStatuses(user, repo, *v.Head.SHA, nil)
			obj = append(obj, Object{
				PullRequest: *v,
				Statuses:    repoStatuses,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
	// return prs, nil
	return obj, nil
}

func fetchIssuesFromGitHub(user, repo string) ([]github.Issue, error) {
	if user == "" || repo == "" {
		return []github.Issue{}, errors.New("user/repo invalid format")
	}
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
