package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gocolly/colly"
)

const (
	delay            = 1 * time.Second
	endpoint         = "http://gb7nb.ddns.net/dstarrepeater/local_tx.php"
	fqdn             = "http://gb7nb.ddns.net/*"
	message_enable   = true
	periodic_enable  = true
	periodic_message = 10
	queued_frequency = 500
	queued_messages  = true
	webhook          = "XXX"
)

var jobqueue []Queue

type Lastheard struct {
	date     string
	callsign string
}

type Queue struct {
	messages []byte
}

type Stats struct {
	checks       int
	sentMessages int
	timer        int
}

// Fire a message to mattermost
func firemsg(m *[]byte) {
	put, _ := http.NewRequest("POST", webhook, bytes.NewBuffer(*m))
	put.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(put)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
}

// Process queued messages
func jobs(q *[]Queue) {
	for i := 0; i < len(*q); i++ {
		firemsg(&(*q)[i].messages)
	}
	jobqueue = nil
}

func main() {
	var job Queue
	var lh []Lastheard
	var msg []byte
	var prev []byte
	var stat Stats

	c := colly.NewCollector()

	c.IgnoreRobotsTxt = true
	c.Limit(&colly.LimitRule{
		DomainGlob: fqdn,
		Delay:      delay,
	})
	c.AllowURLRevisit = true

	// Check for changes every 2 seconds
	for {
		stat.checks++
		c.OnHTML("table", func(e *colly.HTMLElement) {
			e.ForEach("tr", func(k int, row *colly.HTMLElement) {
				var li Lastheard
				if k != 0 {
					row.ForEach("td", func(i int, rd *colly.HTMLElement) {
						switch i {
						case 0:
							li.date = rd.Text
						case 1:
							li.callsign = rd.Text
						default:
							// Do nothing with the rest
						}
					})
					lh = append(lh, li)
				}
			})
		})
		err := c.Visit(endpoint)
		if err != nil {
			fmt.Println("There was an error loading the stats for GB7NB: ", err)
		}
		c.Wait()

		// Check we have usable data
		if len(lh) > 0 {
			msg = []byte(`{"text": "GB7NB LastHeard: ` + lh[0].callsign + `"}`)
		}

		// Clear array
		lh = nil

		// Check if the last heard station callsign is different
		res := bytes.Compare(msg, prev)

		if res != 0 {
			prev = msg
			if message_enable {
				stat.sentMessages++
				if !queued_messages {
					firemsg(&msg)
				} else {
					job.messages = msg
					jobqueue = append(jobqueue, job)
				}
				if periodic_enable && stat.sentMessages%periodic_message == 0 {
					time.Sleep(2 * time.Second)
					msg = []byte(`{"text": "######STATS######\nChecks: ` + strconv.Itoa(stat.checks) + `\nMessages Sent: ` + strconv.Itoa(stat.sentMessages) + `\n######END########"}`)
					firemsg(&msg)
				}
			}
			// For now just post the stats everytime there's a change.
			// TODO: Send this to syslog
			fmt.Println("######STATS######\nChecks: ", stat.checks, "\nMessages Sent: ", stat.sentMessages, "\n######END########")
		}
		stat.timer++
		// Check if we have any messages in the queue
		if stat.timer > queued_frequency/2 && len(jobqueue) > 0 {
			stat.timer = 0
			jobs(&jobqueue)
		}
		time.Sleep(2 * time.Second)
	}
}
