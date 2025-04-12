package nextcloud

import (
	"bytes"
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

type PollOptionCreate struct {
	Timestamp int64 `json:"timestamp"`
	Duration  int   `json:"duration"`
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

func (n *Nextcloud) Url(endpoint string, pollid int) string {
	return fmt.Sprintf("%s/%s/%d/%s", n.Options.Server, "index.php/apps/polls/api/v1.0/poll", pollid, endpoint)
}

func (n *Nextcloud) VotesUrl(pollid int) string {
	return n.Url("votes", pollid)
}

func (n *Nextcloud) PollsUrl(pollid int) string {
	return n.Url("options", pollid)
}

func (n *Nextcloud) Request(url string, requestType string, requestBody []byte) ([]byte, error) {
	client := http.DefaultClient
	r, err := http.NewRequest(requestType, url, bytes.NewBuffer([]byte(requestBody)))
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
	log.Print("Retrieved URL: ", url, " with ", requestType, " - Status Code: ", resp.StatusCode)
	return body, err

}

func (n *Nextcloud) Get(url string) ([]byte, error) {
	return n.Request(url, "GET", nil)
}

func (n *Nextcloud) Users(pollid int) ([]PollUser, error) {
	body, err := n.Get(n.VotesUrl(pollid))
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

func (n *Nextcloud) DeleteOption(o *PollOption) error {
	log.Print("Removing poll option: ", o.Id)
	url := fmt.Sprintf("%s/%s/%d", n.Options.Server, "index.php/apps/polls/api/v1.0/option/", o.Id)
	_, err := n.Request(url, "DELETE", nil)
	if err != nil {
		log.Fatal("Failed to delete option: ", o.Id)
	}
	return err
}

func (n *Nextcloud) CreateOption(pollid int, o *PollOptionCreate) error {
	log.Print("Creating new option: ", o)
	byte, err := json.Marshal(o)
	if err != nil {
		log.Fatal("Could not marshal new option: ", err)
	}
	url := n.Url("option", pollid)
	_, err = n.Request(url, "POST", byte)
	if err != nil {
		log.Fatal("Failed to post new option: ", err)
	}
	return nil
}

func (n *Nextcloud) DeleteOptions(options []PollOption) error {
	for _, option := range options {
		err := n.DeleteOption(&option)
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *Nextcloud) LoadPoll(pollid int) (*PollOptions, error) {
	users, err := n.Users(pollid)
	if err != nil {
		log.Fatal("Failed to load users")
	}
	body, err := n.Get(n.PollsUrl(pollid))
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

// Create new vote options for Friday and Saturdays starting from the last
// option in PollOptions
func AddNewOptions(pollOptions *PollOptions, i int) []PollOptionCreate {
	nextFriday := map[time.Weekday]int{
		time.Saturday:  6,
		time.Sunday:    5,
		time.Monday:    4,
		time.Tuesday:   3,
		time.Wednesday: 2,
		time.Thursday:  1,
		time.Friday:    7,
	}
	nextSaturday := map[time.Weekday]int{
		time.Saturday:  7,
		time.Sunday:    6,
		time.Monday:    5,
		time.Tuesday:   4,
		time.Wednesday: 3,
		time.Thursday:  2,
		time.Friday:    1,
	}
	latestOption := time.Now()
	for _, option := range pollOptions.Options {
		if option.Datetime().After(latestOption) {
			latestOption = option.Datetime()
		}
	}
	// Reset to midnight - ensures full-day poll if no option was active.
	latestOption = time.Date(latestOption.Year(),
		latestOption.Month(),
		latestOption.Day(),
		0, 0, 0,
		latestOption.Nanosecond(),
		latestOption.Location())
	// Set correct multiplier for the first time through the loop
	nextSaturydayMultiplier := nextSaturday[latestOption.Weekday()]
	nextFridayMultiplier := nextFriday[latestOption.Weekday()]
	newOptions := []PollOptionCreate{}
	for range i {
		newFriday := latestOption.Add((time.Duration(nextFridayMultiplier*24) * time.Hour))
		log.Print("Created new option: ", newFriday.Format("2006-01-02"), " ", newFriday.Weekday())
		newSaturday := latestOption.Add((time.Duration(nextSaturydayMultiplier*24) * time.Hour))
		log.Print("Created new option: ", newSaturday.Format("2006-01-02"), " ", newSaturday.Weekday())
		newOptions = append(newOptions, PollOptionCreate{
			Timestamp: newFriday.Unix(),
			Duration:  24 * 60 * 60,
		})
		newOptions = append(newOptions, PollOptionCreate{
			Timestamp: newSaturday.Unix(),
			Duration:  24 * 60 * 60,
		})
		latestOption = newSaturday
		nextFridayMultiplier = 6
		nextSaturydayMultiplier = 7
	}
	return newOptions
}

// This function just returns the options that would be deleted
func DeletePastOptions(o *PollOptions) []PollOption {
	remove := []PollOption{}
	now := time.Now()
	for _, option := range o.Options {
		if option.Datetime().Before(now) {
			remove = append(remove, option)
		}
	}
	return remove
}
