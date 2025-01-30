package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bergmannf/rpgreminder/nextcloud"
	"github.com/bergmannf/rpgreminder/telegram"
)

var msgs []string

type Config struct {
	Nextcloud nextcloud.NextcloudConfig `json:"nextcloud"`
	Telegram  telegram.TelegramConfig   `json:"telegram"`
}

// Load the configuration from the config file - will extend the sensitive
// information from env variables if needed:
// - NEXTCLOUD_TOKEN
// - TELEGRAM_TOKEN
func loadConfiguration(path string) (*Config, error) {
	var opts Config
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(content, &opts)
	if err != nil {
		return nil, err
	}
	if opts.Nextcloud.Token == "" {
		opts.Nextcloud.Token = os.Getenv("NEXTCLOUD_TOKEN")
	}
	if opts.Telegram.Token == "" {
		opts.Telegram.Token = os.Getenv("TELEGRAM_TOKEN")
	}
	return &opts, nil
}

func main() {
	batchMode := flag.Bool("b", false, "Run the bot in batch mode instead of interactive")
	flag.Parse()
	config, err := loadConfiguration("./testconfig.json")
	if err != nil {
		fmt.Println("Could not load configuration: ", err)
	}

	// NEXTCLOUD SETUP
	nextcloudClient := nextcloud.FromConfig(config.Nextcloud)

	// Telegram
	bot, err := telegram.NewBot(&config.Telegram, &nextcloudClient)
	if err != nil {
		log.Fatal("Could not create Telegram bot - exiting")
		return
	}
	if *batchMode {
		opts, err := nextcloudClient.LoadPoll()
		if err != nil {
			return
		}
		weekend := nextcloud.NextWeekend(opts)
		for _, opt := range weekend {
			log.Print(opt.Id, " on: ", opt.Datetime(), " - ", opt.Datetime().Weekday(), " YES: ", opt.Votes.Yes, " MAYBE: ", opt.Votes.Maybe, " NO: ", opt.Votes.No)
		}
	} else {
		bot.Setup()
	}
}
