package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/b4b4r07/travis-bot/travis"
	"github.com/nlopes/slack"
)

var (
	repo = flag.String("repo", "", "Specify github.com repository name")
	user = flag.String("user", "", "Specify github.com user name")
)

var (
	Params slack.PostMessageParameters = slack.PostMessageParameters{
		Markdown:  true,
		Username:  "travis-bot",
		IconEmoji: ":construction_worker:",
	}
	pattern *regexp.Regexp = regexp.MustCompile(`^bot\s+travis\s+(\w+)`)
)

const location = "Asia/Tokyo"

func main() {
	flag.Parse()
	api := slack.New(os.Getenv("SLACK_TOKEN"))
	os.Exit(run(api))
}

func run(api *slack.Client) int {
	if *user == "" || *repo == "" {
		log.Print("user/repo: invalid format")
		return 1
	}

	connected := travis.AuthenticateWithTravis(os.Getenv("TRAVIS_CI_TOKEN"))
	if !connected {
		log.Print("can't connect travis api")
		return 1
	}
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)

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
				if len(pat) < 2 {
					break
				}
				switch pat[1] {
				case "help":
					p := Params
					attachment := slack.Attachment{
						Title: "Usage:",
						Fields: []slack.AttachmentField{
							slack.AttachmentField{
								Title: "status",
								Value: "Get the `user/repo` CI state via Travis API",
							},
							slack.AttachmentField{
								Title: "rebuild",
								Value: "Restart the last build",
							},
						},
					}
					p.Attachments = []slack.Attachment{attachment}
					_, _, err := api.PostMessage(ev.Channel, "", p)
					if err != nil {
						log.Print(err)
						return 1
					}
				case "status":
					resp, err := travis.GetBuildStateOfRepo(*user + "/" + *repo)
					if err != nil {
						log.Print(err)
						return 1
					}
					color := "#D9D9D9"
					switch resp.State {
					case "passed":
						color = "good"
					case "failed":
						color = "danger"
					case "errored":
						color = "warning"
					}
					p := Params
					attachment := slack.Attachment{
						Fallback: "",
						Fields: []slack.AttachmentField{
							slack.AttachmentField{
								Title: "State",
								Value: fmt.Sprintf("<https://github.com/%s|%s>: %s", resp.Name, resp.Name, resp.State),
								Short: true,
							},
							slack.AttachmentField{
								Title: "LastFinished",
								Value: fmt.Sprintf("<%s|%s>", resp.URL, resp.LastFinished.In(jst).Format("2006/01/02 15:04")),
								Short: true,
							},
						},
						Color: color,
					}
					p.Attachments = []slack.Attachment{attachment}
					_, _, err = api.PostMessage(ev.Channel, "", p)
					if err != nil {
						log.Print(err)
						return 1
					}
				case "rebuild":
					attachment := slack.Attachment{}
					id, err := travis.RestartLastBuild(*user + "/" + *repo)
					if err == nil {
						attachment = slack.Attachment{
							Text:  fmt.Sprintf("Restart the last build <https://travis-ci.org/%s/builds/%d|%d> successfully", *user+"/"+*repo, id, id),
							Color: "good",
						}
					} else {
						attachment = slack.Attachment{
							Title: err.Error(),
							Color: "danger",
						}
					}
					p := Params
					p.Attachments = []slack.Attachment{attachment}
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

			case *slack.InvalidAuthEvent:
				log.Print("Invalid credentials")
				return 1
			}
		}
	}
}
