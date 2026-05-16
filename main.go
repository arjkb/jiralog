// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	minsPerHour    = 60
	secondsPerHour = 60 * minsPerHour
	worklogPrefix  = "   > "
)

type TimeLog struct {
	Card         string
	Hours        float64 // hours uploaded this round
	TotalSeconds int     // accumulated total for the card
	Err          error
}

type Task struct {
	Start        int
	Duration     int
	Descriptions []string
	Summary      string
	Link         string
}

func (t *Task) hours() float64 {
	return float64(t.Duration) / float64(minsPerHour)
}

// startedAt() prints the start time as a formatted time string,
// with adequate zero-padding
func (t *Task) startedAt() string {
	h := t.Start / 100
	m := t.Start % 100

	return fmt.Sprintf("%s:%s", fmt.Sprintf("%02d", h), fmt.Sprintf("%02d", m))
}

func (f *TimeLog) totalHours() float64 {
	return float64(f.TotalSeconds) / float64(secondsPerHour)
}

func main() {
	var acceptAll bool = false
	var choice rune
	var wg sync.WaitGroup
	var date time.Time

	yesterday := flag.Bool("yesterday", false, "marks whether the logs are for yesterday")
	flag.Parse()

	if *yesterday {
		date = time.Now().AddDate(0, 0, -1)
	} else {
		date = time.Now()
	}

	tasks := make(map[string]Task)

	config, err := getConfig("config.toml")
	if err != nil {
		log.Fatal(err)
	}

	data, err := os.ReadFile("input.txt")
	if err != nil {
		log.Fatal(err)
	}

	apiUrl, err := url.Parse(config.Baseurl)
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := 0; i < len(lines)-1; i++ {
		card, ok := readCard(lines[i], config.Prefix)
		if !ok {
			// ignore lines that do not have card information
			continue
		}

		startTime, err := readTime(lines[i])
		if err != nil {
			log.Fatal(err)
		}

		endTime, err := readTime(lines[i+1])
		if err != nil {
			log.Fatal(err)
		}

		currDuration, err := computeDuration(startTime, endTime)
		if err != nil {
			log.Fatal(err)
		}

		desc, ok := readDescription(lines[i])
		if !ok {
			log.Fatal("Failed to obtain description")
		}

		t, ok := tasks[card]
		if !ok {
			// seeing this card for the first time; record its start time
			t.Start = startTime

			tempUrl := *apiUrl
			tempUrl.Path = fmt.Sprintf("browse/%s", card)
			t.Link = tempUrl.String()
		}

		t.Descriptions = append(t.Descriptions, desc)
		t.Duration += currDuration
		tasks[card] = t
	}

	fmt.Printf("Read %d cards\n\n", len(tasks))

	// Obtain the worklog summaries of each task.
	summaries := make(chan struct {
		Card string // to get a handle on which task's summary this is
		Task Task
	})

	for card, task := range tasks {
		wg.Add(1)
		go func(card string, config Config, task Task) {
			defer wg.Done()

			summary, err := getWorklogSummary(config.Aikey, config.Model, config.Prompt, strings.Join(task.Descriptions, ". "))
			if err != nil {
				fmt.Println("Failed to produce a summary: ", err)
			}

			task.Summary = summary

			summaries <- struct {
				Card string
				Task Task
			}{card, task}
		}(card, config, task)
	}

	go func() {
		wg.Wait()
		close(summaries)
	}()

	for s := range summaries {
		task := s.Task
		card := s.Card
		tasks[card] = task

		fmt.Printf("▶ [%s] %.2f h (%d mins) | Started at %s\n", card, task.hours(), task.Duration, task.startedAt())
		fmt.Println("  Link: ", task.Link)
		indentedWorklog := worklogPrefix + strings.ReplaceAll(task.Summary, "\n", "\n"+worklogPrefix)
		fmt.Printf("  Worklog:\n%v\n\n", indentedWorklog)
	}

	finalMessage := make(chan string)
	timeLogStatus := make(chan TimeLog)
	finalResult := make(chan TimeLog)

	for card, task := range tasks {
		if !acceptAll {
			fmt.Printf("\nWorklog: %q\n", task.Summary)
			fmt.Printf("Log %.2f h to %s (y/N/a/q)? ", task.hours(), card)
			fmt.Scanf("%c\n", &choice)
			if choice == 'a' || choice == 'A' {
				acceptAll = true
				fmt.Println("\nLogging all remaining records.")
			} else if choice == 'q' || choice == 'Q' {
				fmt.Println("\nQuitting.")
				break
			}
		}

		if choice == 'y' || choice == 'Y' || acceptAll {
			wg.Add(1)

			// uploader
			go func(date time.Time, card string, task Task, config Config, out chan<- TimeLog) {
				var tlStatus TimeLog
				tlStatus.Card = card

				_, err := uploadHourLog(date, card, task.Duration, task.Start, task.Summary, config, apiUrl)
				if err != nil {
					tlStatus.Err = fmt.Errorf("error logging to %s: %v", card, err)
					out <- tlStatus
					return
				}

				tlStatus.Hours = task.hours()
				out <- tlStatus
			}(date, card, task, config, timeLogStatus)

			// get hour log
			go func(config Config, out chan<- TimeLog, inp <-chan TimeLog) {
				timeLog := <-inp

				if timeLog.Err != nil {
					out <- timeLog
					return
				}

				totalSeconds, err := getTimeSpent(timeLog.Card, config)
				if err != nil {
					timeLog.Err = fmt.Errorf("failed to get time spent: %v", err)
					out <- timeLog
					return
				}

				timeLog.TotalSeconds = totalSeconds
				out <- timeLog
			}(config, finalResult, timeLogStatus)

			// print result
			go func(out chan<- string, inp <-chan TimeLog) {
				defer wg.Done()
				result := <-inp
				if result.Err != nil {
					out <- result.Err.Error()
					return
				}
				out <- fmt.Sprintf("%10s %5.2f h uploaded, total spent = %5.2f h", result.Card, result.Hours, result.totalHours())
			}(finalMessage, finalResult)
		}
	}

	// closer
	go func() {
		wg.Wait()
		close(timeLogStatus)
		close(finalResult)
		close(finalMessage)
	}()

	fmt.Println()

	for message := range finalMessage {
		fmt.Println(message)
	}
}

// makeRequest makes the request to the API.
func makeRequest(method string, url string, payload []byte, username string, key string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request object: %v", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(username, key)

	return http.DefaultClient.Do(req)
}
