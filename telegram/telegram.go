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

type ChannelPollMapping struct {
	ChannelId int64 `json:"id"`
	PollId    int   `json:"pollid"`
}

type TelegramConfig struct {
	ChannelsToPolls []ChannelPollMapping `json:"channels"`
	Token           string               `json:"token"`
	Database        string               `json:"database_path"`
}

type TelegramBot struct {
	lock          sync.Mutex
	bot           *telego.Bot
	configuration *TelegramConfig
	nextcloud     *nextcloud.Nextcloud
	db            *MessageDB
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
		log.Fatal("Could not start bot: ", err)
		return nil, err
	}

	db, err := OpenDatabase(config.Database)
	if err != nil {
		log.Fatal("Could not open database path at: ", config.Database)
		return nil, err
	}

	return &TelegramBot{bot: bot, configuration: config, nextcloud: nextcloud, db: db}, nil
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

	// Store any non-command message so it can be summarized later.
	bh.Handle(t.StoreNonCommand, th.AnyMessage())

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
	// TODO: The cleanup should not be needed if messages are no longer only in memory.
}

func (t *TelegramBot) storeMessage(msg *telego.Message) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	res, err := t.db.insert.Exec(msg.MessageID, msg.Chat.ID, time.Unix(msg.Date, 0).UTC(), msg.From.Username, msg.Text, "sent")
	if err != nil {
		log.Fatal("Error when inserting into database.")
	}
	lid, _ := res.LastInsertId()
	log.Print("Sent message inserted into database: ", lid)
	return nil
}

func (t *TelegramBot) StoreNonCommand(ctx *th.Context, update telego.Update) error {
	log.Print("Received non-command message: ", update.Message.Text)
	return t.storeMessage(update.Message)
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
	t.storeMessage(sent)
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
	formatStringHeader := "| %-10s | %-5s | %5s | %5s | %5s | %8s |"
	formatStringOption := "| %-10s | %-5s | %5d | %5d | %5d | %6.2f %% |"
	msgs := []string{fmt.Sprintf(formatStringHeader, "Weekday", "Date", "Yes", "No", "Maybe", "Total")}
	for _, opt := range options {
		timeVotes := opt.Votes.Yes + opt.Votes.Maybe
		allVotes := opt.Votes.Yes + opt.Votes.Maybe + opt.Votes.No
		percent := (float32(timeVotes) / float32(allVotes)) * 100
		msg := fmt.Sprintf(formatStringOption,
			opt.Datetime().Weekday(),
			opt.Datetime().Format("02/01"),
			opt.Votes.Yes,
			opt.Votes.No,
			opt.Votes.Maybe,
			percent,
		)
		log.Print(msg)
		msgs = append(msgs, msg)
	}
	if len(msgs) == 0 {
		msgs = []string{"No votes cast"}
	}
	msg := fmt.Sprintf("```%s```", strings.Join(msgs, "\n"))

	t.Send(update.Message.Chat.ID, msg, true)
	return nil
}

func (t *TelegramBot) DeleteMessagesHandle(ctx *th.Context, update telego.Update) error {
	t.DeleteMessages(update.Message.Chat.ID)
	log.Print("Deleted all messages.")
	return nil
}

// Remove all messages that were send to the given ChannelID
func (t *TelegramBot) DeleteMessages(channelId int64) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	rows, err := t.db.query.Query("SELECT * FROM messages WHERE channelId = ?", channelId)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var message DBMessage
		err := rows.Scan(&message)
		if err != nil {
			log.Printf("Could not deserialize a DB message: %s", err.Error())
			continue
		}
		t.bot.DeleteMessage(context.Background(), tu.Delete(tu.ID(*message.ChannelId), *message.MsgId))
		log.Print("Deleted message: ", *message.MsgId)
	}
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
