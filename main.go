// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/BurntSushi/toml"
)

const (
	minsPerHour    = 60
	secondsPerHour = 60 * minsPerHour
)

type Config struct {
	Username string
	Key      string
	Baseurl  string
	Prefix   string
	Model    string
	Aikey    string
	Prompt   string
}

type TimeLogStatus struct {
	Card    string
	Success bool
	Message string
	Current float64
}

type FinalResult struct {
	Card         string
	Current      float64
	TotalSeconds int
	Message      string
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

func (f *FinalResult) totalHours() float64 {
	return float64(f.TotalSeconds) / float64(secondsPerHour)
}

func main() {
	var acceptAll bool = false
	var choice rune
	var wg sync.WaitGroup

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

	fmt.Printf("Read %d cards\n", len(tasks))

	// Obtain the worklog summaries of each task.
	summaries := make(chan struct {
		Card string // to get a handle on which task's summary this is
		Task Task
	}, len(tasks))

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

	wg.Wait()
	close(summaries)
	for s := range summaries {
		tasks[s.Card] = s.Task

	}

	fmt.Println()

	// Print details of the tasks.
	for card, task := range tasks {
		fmt.Printf("Task\t: %s\n", card)
		fmt.Printf("Link\t: %s\n", task.Link)
		fmt.Printf("Hours\t: %.2f h, started at %4d (%d mins)\n", task.hours(), task.Start, task.Duration)
		fmt.Printf("Worklog\t: %q \n\n", task.Summary)
	}

	fmt.Println()

	finalMessage := make(chan string)
	timeLogStatus := make(chan TimeLogStatus)
	finalResult := make(chan FinalResult)
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
			go func(card string, task Task, config Config, out chan<- TimeLogStatus) {
				var tlStatus TimeLogStatus
				tlStatus.Card = card

				httpStatus, err := uploadHourLog(card, task.Duration, task.Start, task.Summary, config, apiUrl)
				if err != nil {
					tlStatus.Success = false
					tlStatus.Message = fmt.Sprintf("error logging to %s: %v", card, err)
					out <- tlStatus
					return
				}

				tlStatus.Success = true
				tlStatus.Current = task.hours()
				tlStatus.Message = httpStatus
				out <- tlStatus
			}(card, task, config, timeLogStatus)

			// get hour log
			go func(config Config, out chan<- FinalResult, inp <-chan TimeLogStatus) {
				var finalResult FinalResult

				tlStatus := <-inp
				finalResult.Card = tlStatus.Card
				finalResult.Current = tlStatus.Current

				if !tlStatus.Success {
					finalResult.Message = tlStatus.Message
					out <- finalResult
					return
				}

				totalSeconds, err := getTimeSpent(tlStatus.Card, config)
				if err != nil {
					finalResult.Message = fmt.Sprintf("failed to get time spent: %v", err)
					out <- finalResult
					return
				}

				finalResult.TotalSeconds = totalSeconds
				out <- finalResult
			}(config, finalResult, timeLogStatus)

			// print result
			go func(out chan<- string, inp <-chan FinalResult) {
				defer wg.Done()
				result := <-inp
				if result.Message != "" {
					out <- result.Message
					return
				}
				out <- fmt.Sprintf("%10s %5.2f h uploaded, total spent = %5.2f h", result.Card, result.Current, result.totalHours())
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

// computeDuration computes the difference
// between start and end times in minutes.
func computeDuration(start int, end int) (int, error) {
	const maxPossibleTime = 2359
	var minsTillHour int = 0

	if start > end {
		return -1, fmt.Errorf("start-time %d must be at most equal to end-time %d", start, end)
	}

	if start > maxPossibleTime || start < 0 {
		return -1, fmt.Errorf("start-time %d is invalid", start)
	}

	if end > maxPossibleTime || end < 0 {
		return -1, fmt.Errorf("end-time %d is invalid", end)
	}

	startMinutes, endMinutes := start%100, end%100
	if startMinutes > 59 {
		return -1, fmt.Errorf("start-time %d has invalid minutes of %d", start, startMinutes)
	}

	if endMinutes > 59 {
		return -1, fmt.Errorf("end-time %d has invalid minutes of %d", end, endMinutes)
	}

	hoursInBetween := (end / 100) - (start / 100)
	if startMinutes != 0 {
		hoursInBetween--
		minsTillHour = 60 - startMinutes
	}

	return minsTillHour + hoursInBetween*60 + endMinutes, nil
}

// readCard reads the card number if available, from the given line
func readCard(line string, prefix string) (string, bool) {
	idx := strings.Index(line, "#"+prefix)
	if idx == -1 {
		return "", false
	}

	nextSpaceIdx := strings.Index(line[idx:], " ")
	if nextSpaceIdx == -1 {
		return "", false
	}

	lastIdx := idx + nextSpaceIdx

	if !unicode.IsDigit(rune(line[lastIdx-1])) {
		return "", false
	}

	return line[idx+1 : lastIdx], true
}

// readTime returns the time portion from a string.
func readTime(line string) (int, error) {
	num, err := strconv.Atoi(line[:4])
	if num < 0 {
		return -1, fmt.Errorf("negative time read: %d", num)
	}

	minute := num % 100
	if minute > 59 {
		return -1, fmt.Errorf("minute greater than 59: %d", minute)
	}

	return num, err
}

// readDescription returns the description portion from a string
func readDescription(line string) (string, bool) {
	// description is everything from the end of the card number to the end of the line
	idx := strings.Index(line, "#")
	if idx == -1 {
		return "", false
	}

	nextSpaceIdx := strings.Index(line[idx:], " ")
	if nextSpaceIdx == -1 {
		return "", false
	}

	desc := strings.Trim(line[idx+nextSpaceIdx+1:], ". ")
	return desc, true
}

// getConfig reads the specified config file and returns a Config.
func getConfig(filename string) (Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open file: %v", err)
	}

	fileContentBytes, err := io.ReadAll(file)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read file: %v", err)
	}

	var conf Config
	if _, err := toml.Decode(string(fileContentBytes), &conf); err != nil {
		return Config{}, fmt.Errorf("failed to decode file: %v", err)
	}

	return conf, nil
}
