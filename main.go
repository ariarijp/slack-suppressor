package main

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/nlopes/slack"
)

type Config struct {
	Keywords []string
}

func contains(name string, keywords *[]string) bool {
	for _, keyword := range *keywords {
		if name == keyword {
			return true
		}
	}

	return false
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
			var name *string

			ch, _ := api.GetChannelInfo(ev.Channel)
			if ch != nil {
				name = &ch.Name
			}

			grp, _ := api.GetGroupInfo(ev.Channel)
			if grp != nil {
				name = &grp.Name
			}

			if name == nil {
				continue
			} else if !contains(*name, &conf.Keywords) {
				continue
			}

			err = api.SetChannelReadMark(ch.ID, ev.Timestamp)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Println(ev)
		case *slack.RTMError:
			log.Printf("Error: %s\n", ev.Error())
		case *slack.InvalidAuthEvent:
			log.Println("Invalid credentials")
			return
		}
	}
}
