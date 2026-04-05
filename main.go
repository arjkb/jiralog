// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
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
		}

		t.Descriptions = append(t.Descriptions, desc)
		t.Duration += currDuration
		tasks[card] = t

		fmt.Printf("%4d to %4d %10s %5d mins\n",
			startTime,
			endTime,
			card,
			currDuration,
		)
	}

	fmt.Println()

	for card, task := range tasks {
		fmt.Printf("%10s %5d mins   %5.2f h started at %4d\n", card, task.Duration, float64(task.Duration)/float64(minsPerHour), task.Start)
	}

	fmt.Println()

	finalMessage := make(chan string)
	timeLogStatus := make(chan TimeLogStatus)
	finalResult := make(chan FinalResult)
	for card, task := range tasks {
		if !acceptAll {
			fmt.Printf("Log %.2f h to %s (y/N/a/q)? ", float64(task.Duration)/float64(minsPerHour), card)
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

			summary, err := getWorklogSummary(config.Aikey, config.Model, config.Prompt, strings.Join(tasks[card].Descriptions, ". "))
			if err != nil {
				fmt.Println("Failed to produce a summary: ", err)
			}

			// uploader
			go func(card string, minutes int, startTime int, description string, config Config, out chan<- TimeLogStatus) {
				var tlStatus TimeLogStatus
				tlStatus.Card = card

				httpStatus, err := uploadHourLog(card, minutes, startTime, description, config)
				if err != nil {
					tlStatus.Success = false
					tlStatus.Message = fmt.Sprintf("error logging to %s: %v", card, err)
					out <- tlStatus
					return
				}

				tlStatus.Success = true
				tlStatus.Current = float64(minutes) / float64(minsPerHour)
				tlStatus.Message = httpStatus
				out <- tlStatus
			}(card, task.Duration, task.Start, summary, config, timeLogStatus)

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
				out <- fmt.Sprintf("%10s %5.2f h uploaded, total spent = %5.2f h", result.Card, result.Current, float64(result.TotalSeconds)/float64(secondsPerHour))
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
