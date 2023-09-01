package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

var lastRead int64 = math.MaxInt64
var lastNameSent string

type config struct {
	DiscordKey  string `json:"discordKey"`
	FileToWatch string `json:"fileToWatch"`
	NamePrefix  string `json:"namePrefix"`
}

var conf config

func main() {
	if !loadConfig() {
		promptConfig()
	}

	log.Println("Watching file:", conf.FileToWatch)
	for {
		file, err := os.Open(conf.FileToWatch)
		if err != nil {
			log.Fatal(err)
		}
		info, err := file.Stat()
		if err != nil {
			log.Fatal(err)
		}
		if info.Size() > lastRead {
			// read the new bytes
			bytes := make([]byte, info.Size()-lastRead)
			_, err = file.ReadAt(bytes, lastRead)
			if err != nil {
				log.Fatal(err)
			}
			lines := strings.Split(string(bytes), "\n")
			// read lines backwards so we find the most recent one first
			for i := len(lines) - 1; i >= 0; i-- {
				line := lines[i]
				match := regexp.MustCompile(`^\[[\d:]+] Going aboard the (.*)\.\.\.`).FindStringSubmatch(line)
				if len(match) > 0 {
					sendNameUpdate(conf.NamePrefix + match[1])
					break
				}
			}
		}
		lastRead = info.Size()
		time.Sleep(10 * time.Second)
	}
}

const configFile = "nickhelper-config.json"

func loadConfig() bool {
	file, err := os.Open(configFile)
	if err != nil {
		return false
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&conf)
	if err != nil {
		return false
	}
	log.Println("Loaded config:", conf)
	log.Println("Delete the config file '" + configFile + "' to reconfigure.")
	return true
}

func saveConfig() {
	bytes, _ := json.Marshal(conf)
	_ = os.WriteFile(configFile, bytes, 0644)
	log.Println("Saved config:", conf)
}

func promptConfig() {
	keyPrompt := promptui.Prompt{
		Label: "Discord Key",
	}
	var err error
	for conf.DiscordKey == "" {
		color.Yellow("Ask Imaduck for a discord key linked to your account.")
		conf.DiscordKey, err = keyPrompt.Run()
		if err != nil {
			log.Fatal(err)
		}
		if len(conf.DiscordKey) != 36 {
			fmt.Println("Invalid key: Must be a UUID.")
			conf.DiscordKey = ""
		}
	}

	filePrompt := promptui.Prompt{
		Label: "File to watch",
	}
	for conf.FileToWatch == "" {
		color.Yellow("Path where your chat log is sent to. Omit the quotes.")
		conf.FileToWatch, err = filePrompt.Run()
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(conf.FileToWatch); err != nil {
			log.Println(err, conf.FileToWatch)
			conf.FileToWatch = ""
		}

	}

	namePrompt := promptui.Prompt{
		Label: "Name prefix (optional)",
	}
	color.Yellow("Prefix to put infront of the boat name. ie: 'ds - ' for 'ds - The Halibut'")
	conf.NamePrefix, err = namePrompt.Run()
	if err != nil {
		log.Fatal(err)
	}
	saveConfig()
}

type discordNickRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func sendNameUpdate(name string) {
	if name == lastNameSent {
		return
	}
	lastNameSent = name
	log.Println("Sending name update:", name)
	req := discordNickRequest{
		Key:  conf.DiscordKey,
		Name: name,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		log.Println(err)
		return
	}
	res, err := http.Post("https://superquacken.com/discordnick", "text/plain", bytes.NewReader(reqBytes))
	if err != nil {
		log.Println(err)
		return
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		log.Println("Bad status code:", res.StatusCode, '-', string(body))
		return
	}
	log.Println("Name updated successfully.")
}
