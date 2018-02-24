package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/context"

	gcs "cloud.google.com/go/storage"

	"github.com/tucnak/telebot"
)

var (
	apiToken    = flag.String("token", "", "API Token of telegram")
	gcsBucket   = flag.String("gcs_bucket", "", "Google Cloud Storage bucket to backup data")
	userID      = flag.Int("user_id", 0, "authorized user id")
	s1goAddress = "localhost:8080"
	storage     *gcs.Client
	bot         *telebot.Bot
	ctx         = context.Background()
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
	cmdStart   = "./service start"
	cmdStop    = "./service stop"
	cmdRestart = "./service restart"
	cmdDeploy  = "./service deploy"
)

func main() {
	flag.Parse()
	initialize()
	startChatBot()
}

func initialize() {
	var err error
	storage, err = gcs.NewClient(ctx)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("service", []byte(script), 0777)
	if err != nil {
		panic(err)
	}
	// err = execute(cmdDeploy)
	// if err != nil {
	// 	panic(err)
	// }
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
	handle(b, "/deploy", commandHandler(cmdDeploy))
	handle(b, "/start", commandHandler(cmdStart))
	handle(b, "/stop", commandHandler(cmdStop))
	handle(b, "/restart", commandHandler(cmdRestart))
	handle(b, "/backup", handleBackup)
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

func handleBackup(resp response) error {
	if len(*gcsBucket) == 0 {
		return errors.New("GCS upload is not supported")
	}

	execute(cmdStop)
	defer execute(cmdStart)

	objectName := "stage1st_" + time.Now().Format("2006_01_02.gzip")
	log.Printf("Store to file %s", objectName)
	err := execute(fmt.Sprintf("zip %s Stage1st.BoltDB", objectName))
	if err != nil {
		return err
	}
	defer os.Remove(objectName)

	object := storage.Bucket(*gcsBucket).Object(objectName)
	writer := object.NewWriter(ctx)

	file, err := os.Open(objectName)
	if err != nil {
		return err
	}
	defer file.Close()

	size, err := io.Copy(writer, file)
	if err != nil {
		return err
	}
	err = writer.Close()
	if err != nil {
		return err
	}
	resp.Send(fmt.Sprintf("Succss! Backup size: %d", size))
	return nil
}

func commandHandler(command string) func(response) error {
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
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error execute command %s: %s",
			command, err.Error())
	}
	return nil
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
