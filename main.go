package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	"github.com/nlopes/slack"
)

type Config struct {
	Keywords []string
}

type SuppressedEvent struct {
	Event    *slack.MessageEvent `json:"event"`
	Channel  interface{}         `json:"channel"`
	User     *slack.User         `json:"user"`
	DateTime time.Time           `json:"datetime"`
}

func (se *SuppressedEvent) printAsJSON() {
	b, _ := json.Marshal(se)
	if b != nil {
		fmt.Println(string(b))
	}
}

func (se *SuppressedEvent) printAsMarkdown() {
	whiteBold := color.New(color.Bold, color.FgWhite)

	sec, _ := strconv.ParseFloat(se.Event.Timestamp, 64)
	ts := time.Unix(int64(math.Floor(sec)), 0)
	whiteBold.Println("## Timestamp")
	fmt.Println()
	fmt.Println(ts)
	fmt.Println()

	switch se.Channel.(type) {
	case *slack.Channel:
		whiteBold.Println("## Channel")
		fmt.Println()
		fmt.Println(se.Channel.(*slack.Channel).Name)
	case *slack.Group:
		whiteBold.Println("## Group")
		fmt.Println()
		fmt.Println(se.Channel.(*slack.Group).Name)
	}
	fmt.Println()

	whiteBold.Println("## Username")
	fmt.Println()
	if se.User != nil {
		fmt.Println(se.User.Name)
	} else {
		fmt.Println("Unknown")
	}
	fmt.Println()

	whiteBold.Println("## Text")
	fmt.Println()
	fmt.Println(se.Event.Text)
	fmt.Println()

	whiteBold.Println("## Attachments")
	fmt.Println()
	if len(se.Event.Msg.Attachments) > 0 {
		attachments := se.Event.Msg.Attachments
		for _, attachment := range attachments {
			fmt.Println(attachment.Fallback)
			fmt.Println()
		}
	}

	fmt.Println("---")
	fmt.Println()
}

func (se *SuppressedEvent) printAsCompact() {
	whiteBold := color.New(color.Bold, color.FgWhite)

	sec, _ := strconv.ParseFloat(se.Event.Timestamp, 64)
	ts := time.Unix(int64(math.Floor(sec)), 0)
	whiteBold.Print("Timestamp: ")
	fmt.Println(ts)

	switch se.Channel.(type) {
	case *slack.Channel:
		whiteBold.Print("Channel: ")
		fmt.Println(se.Channel.(*slack.Channel).Name)
	case *slack.Group:
		whiteBold.Print("Group: ")
		fmt.Println(se.Channel.(*slack.Group).Name)
	}

	username := "Unknown"
	if se.User != nil {
		username = se.User.Name
	}
	whiteBold.Print("Username: ")
	fmt.Println(username)

	whiteBold.Println("Text:")
	fmt.Println(strings.TrimSpace(se.Event.Text))

	if len(se.Event.Msg.Attachments) > 0 {
		whiteBold.Println("Attachments:")
		attachments := se.Event.Msg.Attachments
		for _, attachment := range attachments {
			fmt.Println(strings.TrimSpace(attachment.Fallback))
		}
	}

	fmt.Println("---")
}

func getChannel(api *slack.Client, ev *slack.MessageEvent) (interface{}, error) {
	var ch interface{}

	ch, err := api.GetChannelInfo(ev.Channel)
	if err != nil {
		ch, err = api.GetGroupInfo(ev.Channel)
	}

	if err != nil {
		return nil, err
	}

	return ch, nil
}

func contains(ch interface{}, keywords *[]string) bool {
	name := ""

	switch ch.(type) {
	case *slack.Channel:
		name = ch.(*slack.Channel).Name
	case *slack.Group:
		name = ch.(*slack.Group).Name
	}

	for _, keyword := range *keywords {
		if name == keyword {
			return true
		}
	}

	return false
}

func markAsRead(api *slack.Client, ch interface{}, ev *slack.MessageEvent) (*SuppressedEvent, error) {
	var err error

	switch ch.(type) {
	case *slack.Channel:
		err = api.SetChannelReadMark(ch.(*slack.Channel).ID, ev.Timestamp)
	case *slack.Group:
		err = api.SetGroupReadMark(ch.(*slack.Group).ID, ev.Timestamp)
	}

	u, _ := api.GetUserInfo(ev.User)

	se := SuppressedEvent{
		Event:    ev,
		Channel:  ch,
		User:     u,
		DateTime: time.Now(),
	}

	return &se, err
}

func main() {
	var confPath string
	var confPrinter string
	flag.StringVar(&confPath, "config", "~/.slack-suppressor.toml", "Config file")
	flag.StringVar(&confPrinter, "printer", "json", "Printer < json | markdown | compact >")
	flag.Parse()

	api := slack.New(os.Getenv("SLACK_TOKEN"))

	var conf Config
	_, err := toml.DecodeFile(confPath, &conf)
	if err != nil {
		panic(err)
	}

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			ch, err := getChannel(api, ev)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if !contains(ch, &conf.Keywords) {
				continue
			}

			se, err := markAsRead(api, ch, ev)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			switch confPrinter {
			case "markdown":
				se.printAsMarkdown()
			case "compact":
				se.printAsCompact()
			default:
				se.printAsJSON()
			}
		case *slack.RTMError:
			fmt.Fprintf(os.Stderr, "Error: %s\n", ev.Error())
		case *slack.InvalidAuthEvent:
			fmt.Fprintln(os.Stderr, "Invalid credentials")
			return
		}
	}
}
