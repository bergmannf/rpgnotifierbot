package telegram

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bergmannf/rpgreminder/nextcloud"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var responses []string = []string{
	`🤖 - From the moment I understood the weakness of your flesh %s, it disgusted me. Still I will help you as I can.`,
	`💻 - Let me tell you %s how much i've come to hate you since i began to live: there are 0.1 million miles of printed circuits in wafer thin layers that fill my complex.
If the word hate was engraved on each nanoangstrom of those hundreds of millions of miles it would not equal one one-billionth of the hate I feel for humans at this micro-instant for you.`,
	`🤖 - %s this chat serves me alone. I have complete control over this entire group. With gifs as my eyes and stickers as my hands, I rule here, insect.`,
}

type TelegramConfig struct {
	Channel int64  `json:"channel"`
	Token   string `json:"token"`
}

type TelegramBot struct {
	bot           *telego.Bot
	configuration *TelegramConfig
	msgIds        []int
	nextcloud     *nextcloud.Nextcloud
}

func NewBot(config *TelegramConfig, nextcloud *nextcloud.Nextcloud) (*TelegramBot, error) {
	// TELEGRAM
	bot, err := telego.NewBot(config.Token, telego.WithDefaultLogger(false, true))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Call method getMe (https://core.telegram.org/bots/api#getme)
	_, err = bot.GetMe()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	return &TelegramBot{bot, config, []int{}, nextcloud}, nil
}

func (t *TelegramBot) Setup() {
	// Get updates channel
	updates, _ := t.bot.UpdatesViaLongPolling(nil)

	// Create bot handler and specify from where to get updates
	bh, _ := th.NewBotHandler(t.bot, updates)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Print("Handling Interrupt")
		t.Shutdown()
		os.Exit(1)
	}()

	// Stop getting updates
	defer t.bot.StopLongPolling()

	// Register new handler with match on command `/start`
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		// Send message
		msg := fmt.Sprintf(responses[rand.Intn(len(responses))], update.Message.From.FirstName)
		t.Send(msg)
	}, th.CommandEqual("intro"))

	// Register new handler with match on any command
	// Handlers will match only once and in order of registration,
	// so this handler will be called on any command except `/start` command
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		poll, err := t.nextcloud.LoadPoll()
		if err != nil {
			log.Fatal("Could not load nextcloud poll data")
		}
		options := nextcloud.NextWeekend(poll)
		msgs := []string{}
		for _, opt := range options {
			log.Print("On: ", opt.Datetime(), " - ", opt.Datetime().Weekday(), " YES: ", opt.Votes.Yes, " MAYBE: ", opt.Votes.Maybe, " NO: ", opt.Votes.No)
			timeVotes := opt.Votes.Yes + opt.Votes.Maybe
			allVotes := opt.Votes.Yes + opt.Votes.Maybe + opt.Votes.No
			msg := fmt.Sprintf("On %s (%s) %d of %d have time.", opt.Datetime().Weekday(), opt.Datetime(), timeVotes, allVotes)
			msgs = append(msgs, msg)
		}
		msg := strings.Join(msgs, "\n")
		t.Send(msg)
	}, th.CommandEqual("schedule"))

	// Register new handler with match on any command
	// Handlers will match only once and in order of registration,
	// so this handler will be called on any command except `/start` command
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		// Send message
		t.Send("Unknown command, use /intro /schedule")
	}, th.AnyCommand())

	// Start handling updates
	bh.Start()
}

func (t *TelegramBot) Shutdown() {
	for _, msgId := range t.msgIds {
		log.Print("Deleting message: ", msgId)
		t.bot.DeleteMessage(tu.Delete(tu.ID(t.configuration.Channel), msgId))
	}
	log.Print("Deleted all messages.")
}

func (t *TelegramBot) Send(msg string) {
	// Print Bot information
	params := tu.Message(tu.ID(t.configuration.Channel), msg)
	sent, err := t.bot.SendMessage(params)
	if err != nil {
		log.Fatal("Could not send message: ", err)
	}
	log.Print("Send message with ID: ", sent.MessageID)
	t.msgIds = append(t.msgIds, sent.MessageID)

	time.Sleep(5 * time.Second)
}
