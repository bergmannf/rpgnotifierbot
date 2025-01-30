package nextcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"time"
)

type NextcloudConfig struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Token    string `json:"token"`
	PollId   int    `json:"pollid"`
}

type PollVote struct {
	Id         int      `json:"id"`
	PollId     int      `json:"pollId"`
	OptionText string   `json:"optionText"`
	Answer     string   `json:"answer"`
	Deleted    int      `json:"deleted"`
	OptionId   int      `json:"optionId"`
	User       PollUser `json:"user"`
}

type PollVotes struct {
	Options []PollVote `json:"votes"`
}

type PollOptionVote struct {
	No          int    `json:"no"`
	Yes         int    `json:"yes"`
	Maybe       int    `json:"maybe"`
	Count       int    `json:"count"`
	CurrentUser string `json:"currentUser"`
}

type PollUser struct {
	Id           string `json:"id"`
	UserId       string `json:"userId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	IsNoUser     bool   `json:"isNoUser"`
	Type         string `json:"type"`
}

type PollOption struct {
	Id        int            `json:"id"`
	PollId    int            `json:"pollId"`
	Text      string         `json:"text"`
	Timestamp int64          `json:"timestamp"`
	Deleted   int            `json:"deleted"`
	Order     int            `json:"order"`
	Confirmed int            `json:"confirmed"`
	Duration  int            `json:"duration"`
	Locked    bool           `json:"locked"`
	Hash      string         `json:"hash"`
	Votes     PollOptionVote `json:"votes"`
	Owner     PollUser       `json:"owner"`
}

func (o *PollOption) Datetime() time.Time {
	return time.Unix(o.Timestamp, 0)
}

type PollOptions struct {
	Options []PollOption `json:"options"`
}

type Nextcloud struct {
	Options NextcloudConfig
}

func FromConfig(opts NextcloudConfig) Nextcloud {
	return Nextcloud{Options: opts}
}

func (n *Nextcloud) Url(endpoint string) string {
	return fmt.Sprintf("%s/%s/%d/%s", n.Options.Server, "index.php/apps/polls/api/v1.0/poll", n.Options.PollId, endpoint)
}

func (n *Nextcloud) VotesUrl() string {
	return n.Url("votes")
}

func (n *Nextcloud) PollsUrl() string {
	return n.Url("options")
}

func (n *Nextcloud) Get(url string) ([]byte, error) {
	client := http.DefaultClient
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	r.SetBasicAuth(n.Options.Username, n.Options.Token)
	r.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(r)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Print("Retrieved URL: ", n.PollsUrl(), " Status Code: ", resp.StatusCode)
	return body, err
}

func (n *Nextcloud) Users() ([]PollUser, error) {
	body, err := n.Get(n.VotesUrl())
	var votes PollVotes
	err = json.Unmarshal(body, &votes)
	if err != nil {
		log.Fatal("Could not unmarshal nextcloud poll: ", err)
	}
	var users []PollUser
	for _, vote := range votes.Options {
		if !slices.Contains(users, vote.User) {
			users = append(users, vote.User)
		}
	}
	return users, nil
}

func (n *Nextcloud) LoadPoll() (*PollOptions, error) {
	users, err := n.Users()
	if err != nil {
		log.Fatal("Failed to load users")
	}
	body, err := n.Get(n.PollsUrl())
	if err != nil {
		log.Fatal("Failed to load options")
	}
	var options PollOptions
	err = json.Unmarshal(body, &options)
	if err != nil {
		log.Fatal("Could not unmarshal nextcloud poll: ", err)
	}
	for i, _ := range options.Options {
		yes := options.Options[i].Votes.Yes
		maybe := options.Options[i].Votes.Maybe
		options.Options[i].Votes.No = len(users) - (yes + maybe)
	}
	return &options, nil
}

// Return the dates of the next weekend only
func NextWeekend(options *PollOptions) []PollOption {
	var nextWeekend []PollOption
	today := time.Now()
	for _, opt := range options.Options {
		date := opt.Datetime()
		diff := date.Sub(today)
		if diff.Hours() > 0 && diff.Hours() < 24*7 {
			nextWeekend = append(nextWeekend, opt)
		}
	}
	return nextWeekend
}
