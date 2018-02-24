package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/tucnak/telebot"
)

var (
	apiToken    = flag.String("token", "", "API Token of telegram")
	userID      = flag.Int("user_id", 0, "authorized user id")
	s1goAddress = "localhost:8080"
)

const (
	script = `#!/bin/bash

start_s1go() {
	./s1go --interval=1800 --username=$S1GOUSERNAME \
			--password=$S1GOPASSWORD >> fetch.log 2>&1 &
}

kill_s1go() {
	pkill "^s1go\$"
}

build_s1go() {
	go get -u -v -t github.com/smy20011/s1go
	go build github.com/smy20011/s1go
}

case "$1" in
	start)   start_s1go ;;
	stop)    kill_s1go ;;
	restart) kill_s1go; start_s1go ;;
	deploy)  build_s1go; kill_s1go; start_s1go ;;
	*) exit 1
esac`
)

func main() {
	flag.Parse()
	initialize()
	startChatBot()
}

func initialize() {
	err := ioutil.WriteFile("service", []byte(script), 0777)
	if err != nil {
		panic(err)
	}
	err = execute("./service deploy")
	if err != nil {
		panic(err)
	}
}

func startChatBot() {
	b, err := telebot.NewBot(telebot.Settings{
		Token:  *apiToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		panic(err)
	}

	handle(b, "/status", handleStatus)
	handle(b, "/deploy", serviceHandler("./service deploy"))
	handle(b, "/start", serviceHandler("./service start"))
	handle(b, "/stop", serviceHandler("./service stop"))
	handle(b, "/restart", serviceHandler("./service restart"))
	b.Start()
}

func handleStatus(response response) (err error) {
	resp, err := http.DefaultClient.Get(fmt.Sprintf("http://%s/debug/vars", s1goAddress))
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	result := struct {
		LastFetchTime int64 `json:"crawler/lastfetchtime"`
	}{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return
	}
	fmt.Printf("Timestamp: %v", result)
	response.Send(fmt.Sprintf("Last update time: %s",
		time.Unix(result.LastFetchTime, 0).Format(time.UnixDate)))
	return nil
}

func serviceHandler(command string) func(response) error {
	return func(resp response) error {
		err := execute(command)
		if err == nil {
			resp.Send("Success!")
		}
		return err
	}
}

func execute(command string) error {
	commands := strings.Split(command, " ")
	cmd := exec.Command(commands[0], commands[1:]...)
	return cmd.Run()
}

type response interface {
	Send(message string)
}

type responseImpl struct {
	bot     *telebot.Bot
	message *telebot.Message
}

func (r responseImpl) Send(m string) {
	r.bot.Send(r.message.Sender, m)
}

func handle(bot *telebot.Bot, command string, handler func(response) error) {
	bot.Handle(command, func(m *telebot.Message) {
		response := &responseImpl{bot, m}
		if m.Sender.ID != *userID {
			response.Send(fmt.Sprintf("Unauthorized user: %d", m.Sender.ID))
		} else {
			err := handler(response)
			if err != nil {
				response.Send(err.Error())
			}
		}
	})
}
