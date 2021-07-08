package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/gocolly/colly"
)

var (
	delay    = 1 * time.Second
	endpoint = "http://gb7nb.ddns.net/dstarrepeater/local_tx.php"
	fqdn     = "http://gb7nb.ddns.net/*"
	webhook  = "XXX"
)

type Lastheard struct {
	date     string
	callsign string
}

type Stats struct {
	checks       int64
	sentMessages int64
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

func main() {
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
			firemsg(&msg)
			stat.sentMessages++
			// For now just post the stats everytime there's a change.
			// TODO: Send this to syslog
			fmt.Println("######STATS######\nChecks: ", stat.checks, "\nMessages Sent: ", stat.sentMessages, "\n######END########")
		}
		time.Sleep(2 * time.Second)
	}
}
