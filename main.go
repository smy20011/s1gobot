package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/tucnak/telebot"
)

var (
	apiToken    = flag.String("token", "", "API Token of telegram")
	userID      = flag.Int("user_id", 0, "authorized user id")
	s1goAddress = flag.String("s1go_address", "", "Address of s1go listening server.")
)

func main() {
	flag.Parse()
	b, err := telebot.NewBot(telebot.Settings{
		Token:  *apiToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		panic(err)
	}

	handle(b, "/status", func(_ string) string {
		resp, err := http.DefaultClient.Get(fmt.Sprintf("http://%s/debug/vars", *s1goAddress))
		if err != nil {
			return err.Error()
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err.Error()
		}
		result := struct {
			LastFetchTime int64 `json:"crawler/lastfetchtime"`
		}{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err.Error()
		}
		fmt.Printf("Timestamp: %v", result)
		return fmt.Sprintf("Last update time: %s",
			time.Unix(result.LastFetchTime, 0).Format(time.UnixDate))
	})

	b.Start()
}

func handle(bot *telebot.Bot, command string, handler func(string) string) {
	bot.Handle(command, func(m *telebot.Message) {
		if m.Sender.ID != *userID {
			bot.Send(m.Sender, fmt.Sprintf("Unauthorized user: %d", m.Sender.ID))
		} else {
			bot.Send(m.Sender, handler(m.Text))
		}
	})
}
