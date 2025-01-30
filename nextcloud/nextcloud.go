package nextcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type NextcloudConfig struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Token    string `json:"token"`
	PollId   int    `json:"pollid"`
}

type PollVote struct {
	No          int    `json:"no"`
	Yes         int    `json:"yes"`
	Maybe       int    `json:"maybe"`
	Count       int    `json:"count"`
	CurrentUser string `json:"currentUser"`
}

type PollOwner struct {
	Id           string `json:"id"`
	UserId       string `json:"userId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	IsNoUser     bool   `json:"isNoUser"`
	Type         string `json:"type"`
}

type PollOption struct {
	Id        int       `json:"id"`
	PollId    int       `json:"pollId"`
	Text      string    `json:"text"`
	Timestamp int64     `json:"timestamp"`
	Deleted   int       `json:"deleted"`
	Order     int       `json:"order"`
	Confirmed int       `json:"confirmed"`
	Duration  int       `json:"duration"`
	Locked    bool      `json:"locked"`
	Hash      string    `json:"hash"`
	Votes     PollVote  `json:"votes"`
	Owner     PollOwner `json:"owner"`
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

func (n *Nextcloud) PollsUrl() string {
	return fmt.Sprintf("%s/%s/%d/%s", n.Options.Server, "index.php/apps/polls/api/v1.0/poll", n.Options.PollId, "options")
}

func (n *Nextcloud) LoadPoll() (*PollOptions, error) {
	client := http.DefaultClient
	r, err := http.NewRequest("GET", n.PollsUrl(), nil)
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
	var options PollOptions
	err = json.Unmarshal(body, &options)
	if err != nil {
		log.Fatal("Could not unmarshal nextcloud poll: ", err)
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
