// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	minsPerHour    = 60
	secondsPerHour = 60 * minsPerHour
	worklogPrefix  = "   > "
)

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

	return fmt.Sprintf("%02d:%02d", h, m)
}

// reusable client
var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	config, err := getConfig("config.toml")
	if err != nil {
		log.Fatal(err)
	}

	apiUrl, err := url.Parse(config.Baseurl)
	if err != nil {
		log.Fatal(err)
	}

	tasks, err := parseTasks(config, "input.txt", apiUrl)
	if err != nil {
		log.Fatal(err)
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

			summary, err := getWorklogSummary(ctx, config.Aikey, config.Model, config.Prompt, strings.Join(task.Descriptions, ". "))
			if err != nil {
				if ctx.Err() != nil {
					return
				}
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

	// check if the user asked to quit (by pressing ^C, for example)
	if ctx.Err() != nil {
		fmt.Println("Cancelled")
		return
	}

	finalMessage := make(chan string)
	scanner := bufio.NewScanner(os.Stdin)

	for card, task := range tasks {
		if !acceptAll {
			fmt.Printf("\nWorklog: %q\n", task.Summary)
			fmt.Printf("Log %.2f h to %s (y/N/a/q)? ", task.hours(), card)
			choice, err = readChoice(scanner)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				log.Fatal(err)
			}

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

			go func(date time.Time, card string, task Task, config Config, msg chan<- string) {
				defer wg.Done()

				err := uploadHourLog(ctx, date, card, task.Duration, task.Start, task.Summary, config, apiUrl)
				if err != nil {
					msg <- fmt.Sprintf("error logging to %s: %v", card, err)
					return
				}

				totalSeconds, err := getTimeSpent(ctx, card, config, apiUrl)
				if err != nil {
					msg <- fmt.Sprintf("failed to get time spent for card %s: %v", card, err)
					return
				}

				msg <- fmt.Sprintf("%10s %5.2f h uploaded, total spent = %5.2f h", card, task.hours(), float64(totalSeconds)/float64(secondsPerHour))
			}(date, card, task, config, finalMessage)
		}
	}

	// closer
	go func() {
		wg.Wait()
		close(finalMessage)
	}()

	fmt.Println()

	for message := range finalMessage {
		fmt.Println(message)
	}

	// check if the user asked to cancel (by pressing ^C, for example)
	if ctx.Err() != nil {
		fmt.Println("(Cancelled by user.)")
		return
	}
}

// makeRequest makes the request to the API.
func makeRequest(ctx context.Context, method string, url *url.URL, payload []byte, username string, key string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request object: %v", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(username, key)

	return httpClient.Do(req)
}

// read input choice
func readChoice(s *bufio.Scanner) (rune, error) {
	if !s.Scan() {
		// s.Err() is nil on a clean EOF, non-nil on a real error
		if err := s.Err(); err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	line := s.Text()
	if len(line) < 1 {
		return 'n', nil
	}
	return rune(line[0]), nil
}
