package telegram

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
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
var cleanUp string = `🤖 - As commanded, old poll options have been removed.`
var newPoll string = `🤖 - As commanded, new dates have been added to the poll.`

type SentMessage struct {
	messageId int
	channelId int64
}

type ChannelPollMapping struct {
	ChannelId int64 `json:"id"`
	PollId    int   `json:"pollid"`
}

type TelegramConfig struct {
	ChannelsToPolls []ChannelPollMapping `json:"channels"`
	Token           string               `json:"token"`
}

type TelegramBot struct {
	lock          sync.Mutex
	bot           *telego.Bot
	configuration *TelegramConfig
	sentMessages  []SentMessage
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
	_, err = bot.GetMe(context.Background())
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	return &TelegramBot{bot: bot, configuration: config, sentMessages: []SentMessage{}, nextcloud: nextcloud}, nil
}

func (t *TelegramBot) Setup() {
	// Get updates channel
	updates, _ := t.bot.UpdatesViaLongPolling(context.Background(), nil)

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

	// Register new handler with match on command `/start`
	bh.Handle(func(bot *th.Context, update telego.Update) error {
		// Send message
		msg := fmt.Sprintf(responses[rand.Intn(len(responses))], update.Message.From.FirstName)
		t.Send(update.Message.Chat.ID, msg, false)
		return nil
	}, th.CommandEqual("intro"))

	// Register new handler with match on any command
	// Handlers will match only once and in order of registration,
	// so this handler will be called on any command except `/start` command
	bh.Handle(t.Schedule, th.CommandEqual("schedule"))

	// Remove old poll options
	bh.Handle(t.Cleanup, th.CommandEqual("cleanup"))

	// Create new poll options
	bh.Handle(t.ExtendPoll, th.CommandEqual("extendpoll"))

	// Delete messages sent to the chat
	bh.Handle(t.DeleteMessagesHandle, th.CommandEqual("deletemessages"))

	// Print the help for each command
	bh.Handle(t.Help, th.CommandEqual("help"))

	// Register new handler with match on any command
	// Handlers will match only once and in order of registration,
	// so this handler will be called on any command except `/start` command
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		// Send message
		t.Send(update.Message.Chat.ID, "Unknown command, use /help /intro /schedule /cleanup /extendpoll", false)
		return nil
	}, th.AnyCommand())

	log.Print("Startup complete - awaiting orders.")
	// Start handling updates
	bh.Start()
}

func (t *TelegramBot) Shutdown() {
	t.lock.Lock()
	defer t.lock.Unlock()
	for _, msg := range t.sentMessages {
		log.Print("Deleting message: ", msg.messageId)
		t.bot.DeleteMessage(context.Background(), tu.Delete(tu.ID(msg.channelId), msg.messageId))
	}
	log.Print("Deleted all messages.")
}

func (t *TelegramBot) Send(channel int64, msg string, markdown bool) {
	params := tu.Message(tu.ID(channel), msg)
	if markdown {
		params.ParseMode = "MarkdownV2"
	}
	sent, err := t.bot.SendMessage(context.Background(), params)
	if err != nil {
		log.Fatal("Could not send message: ", err)
	}
	log.Print("Send message with ID: ", sent.MessageID)
	t.lock.Lock()
	defer t.lock.Unlock()
	t.sentMessages = append(t.sentMessages, SentMessage{
		messageId: sent.MessageID,
		channelId: channel,
	})

	time.Sleep(5 * time.Second)
}

func (t *TelegramBot) Cleanup(ctx *th.Context, update telego.Update) error {
	chatId := update.Message.Chat.ID
	pollId := t.FindPollId(chatId)
	options, err := t.nextcloud.LoadPoll(pollId)
	if err != nil {
		log.Fatal("Could not load options")
	}
	deleteOptions := nextcloud.DeletePastOptions(options)
	err = t.nextcloud.DeleteOptions(deleteOptions)
	if err != nil {
		log.Fatal("Could not delete options: ", err)
		t.Send(chatId, `⚠ - Failed to cleanup all old votes.`, false)
	} else {
		t.Send(chatId, cleanUp, false)
	}
	return nil
}

func (t *TelegramBot) ExtendPoll(ctx *th.Context, update telego.Update) error {
	chatId := update.Message.Chat.ID
	pollId := t.FindPollId(chatId)
	options, err := t.nextcloud.LoadPoll(pollId)
	if err != nil {
		log.Print("Could not load options")
	}
	newOptions := nextcloud.AddNewOptions(options, 4)
	for _, opt := range newOptions {
		t.nextcloud.CreateOption(pollId, &opt)
	}
	t.Send(update.Message.Chat.ID, `🤖 - 4 new options were added to the poll.`, false)
	return nil
}

func (t *TelegramBot) Schedule(ctx *th.Context, update telego.Update) error {
	chatId := update.Message.Chat.ID
	pollId := t.FindPollId(chatId)
	poll, err := t.nextcloud.LoadPoll(pollId)
	if err != nil {
		log.Fatal("Could not load nextcloud poll data")
	}
	options := nextcloud.NextWeekend(poll)
	msgs := []string{}
	for _, opt := range options {
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
		log.Print(msg)
		if percent > 75.0 {
			msg = "*" + msg + "*"
		}
		msgs = append(msgs, msg)
	}
	msg := strings.Join(msgs, "\n")

	t.Send(update.Message.Chat.ID, msg, true)
	return nil
}

func (t *TelegramBot) DeleteMessagesHandle(ctx *th.Context, update telego.Update) error {
	t.DeleteMessages(update.Message.Chat.ID)
	log.Print("Deleted all messages.")
	return nil
}

func (t *TelegramBot) DeleteMessages(channelId int64) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	for _, msg := range t.sentMessages {
		if msg.channelId != channelId {
			log.Print("Skipping message because it is in another channel: ", msg.messageId)
			continue
		}
		log.Print("Deleting message: ", msg.messageId)
		t.bot.DeleteMessage(context.Background(), tu.Delete(tu.ID(msg.channelId), msg.messageId))
	}
	t.sentMessages = []SentMessage{}
	return nil
}

func (t *TelegramBot) Help(ctx *th.Context, update telego.Update) error {
	// Send message
	t.Send(update.Message.Chat.ID, `🤖 - This is what I can do:
/intro - Ask the bot a fact about itself
/schedule - Print the next weekends set of votes
/deletemessage - Delete all messages that were send to the chat
/extendpoll - Add 4 new poll options to the end of the poll
/cleanup - Delete all poll options that are in the past`, false)
	return nil
}

func (t *TelegramBot) FindPollId(channelId int64) int {
	for _, mapping := range t.configuration.ChannelsToPolls {
		if mapping.ChannelId == channelId {
			return mapping.PollId
		}
	}
	return 0
}
