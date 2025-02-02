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
	configFile := flag.String("c", "/etc/rpgreminder/config.json", "Configuration file for the bot")
	flag.Parse()
	config, err := loadConfiguration(*configFile)
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
		msgs := []string{}
		for _, opt := range weekend {
			timeVotes := opt.Votes.Yes + opt.Votes.Maybe
			allVotes := opt.Votes.Yes + opt.Votes.Maybe + opt.Votes.No
			percent := (float32(timeVotes) / float32(allVotes)) * 100
			msg := fmt.Sprintf(`%s \(%s\): %d \(YES\) %d \(MAYBE\) %d \(NO\): %.0f%%`,
				opt.Datetime().Weekday(),
				opt.Datetime().Format("02/01"),
				opt.Votes.Yes,
				opt.Votes.Maybe,
				opt.Votes.No,
				percent,
			)
			if percent > 75.0 {
				msg = "*" + msg + "*"
			}
			log.Print(msg)
			msgs = append(msgs, msg)
		}
		deletionOptions := nextcloud.DeletePastOptions(opts)
		for _, opt := range deletionOptions {
			log.Print("Would delete options: ", opt)
			// nextcloudClient.DeleteOption(&opt)
		}
		newOptions := nextcloud.AddNewOptions(opts, 2)
		for _, opt := range newOptions {
			log.Print("New options: ", opt)
			// nextcloudClient.CreateOption(&opt)
		}
		// bot.Send(strings.Join(msgs, "\n"))
	} else {
		bot.Setup()
	}
}
