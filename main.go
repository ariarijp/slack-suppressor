package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/nlopes/slack"
)

type Config struct {
	Keywords []string
}

type SuppressedEvent struct {
	Event   *slack.MessageEvent `json:"event"`
	Channel interface{}         `json:"channel"`
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

func markAsRead(api *slack.Client, ch interface{}, ev *slack.MessageEvent) error {
	var err error

	switch ch.(type) {
	case *slack.Channel:
		err = api.SetChannelReadMark(ch.(*slack.Channel).ID, ev.Timestamp)
	case *slack.Group:
		err = api.SetGroupReadMark(ch.(*slack.Group).ID, ev.Timestamp)
	}

	b, _ := json.Marshal(SuppressedEvent{
		Event:   ev,
		Channel: ch,
	})
	if b != nil {
		fmt.Println(string(b))
	}

	return err
}

func main() {
	api := slack.New(os.Getenv("SLACK_TOKEN"))

	var conf Config
	_, err := toml.DecodeFile("config.toml", &conf)
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

			err = markAsRead(api, ch, ev)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		case *slack.RTMError:
			fmt.Fprintf(os.Stderr, "Error: %s\n", ev.Error())
		case *slack.InvalidAuthEvent:
			fmt.Fprintln(os.Stderr, "Invalid credentials")
			return
		}
	}
}
