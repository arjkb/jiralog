// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
)

const minsPerHour = 60

type Config struct {
	Username string
	Key      string
	Baseurl  string
	Prefix   string
}

type Payload struct {
	Started          string `json:"started"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
}

type Status struct {
	Card             string
	Logged           bool
	LogStatusMessage string
	Current          float64
	Total            string
}

func main() {
	var choice rune
	var wg sync.WaitGroup

	durations := make(map[string]int)
	startTimes := make(map[string]int)

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
		startTime, err := readTime(lines[i])
		if err != nil {
			log.Fatal(err)
		}

		endTime, err := readTime(lines[i+1])
		if err != nil {
			log.Fatal(err)
		}

		card, ok := readCard(lines[i], config.Prefix)
		if !ok {
			// ignore lines that do not have card information
			continue
		}

		currDuration, err := computeDuration(startTime, endTime)
		if err != nil {
			log.Fatal(err)
		}

		durationSum, ok := durations[card]
		if !ok {
			startTimes[card] = startTime
		}

		durations[card] = durationSum + currDuration

		fmt.Printf("%4d to %4d %10s %5d mins\n", startTime, endTime, card, currDuration)
	}

	fmt.Println()

	for card, duration := range durations {
		fmt.Printf("%10s %5d mins   %5.2f h started at %4d\n", card, duration, float64(duration)/float64(minsPerHour), startTimes[card])
	}

	fmt.Println()

	finalMessage := make(chan string)
	hourLogStatus := make(chan Status)
	for card, duration := range durations {
		fmt.Printf("Log %.2f h to %s (y/N)? ", float64(duration)/float64(minsPerHour), card)
		fmt.Scanf("%c\n", &choice)
		if choice == 'y' || choice == 'Y' {
			wg.Add(1)

			// uploader
			go func(card string, minutes int, startTime int, config Config, out chan<- Status) {
				var status Status
				status.Card = card

				statusMessage, err := uploadHourLog(card, duration, startTime, config)
				if err != nil {
					status.Logged = false
					status.LogStatusMessage = fmt.Sprintf("error logging to %s: %v", card, err)
					out <- status
					return
				}

				status.Logged = true
				status.Current = float64(duration) / float64(minsPerHour)
				status.LogStatusMessage = statusMessage
				out <- status
			}(card, duration, startTimes[card], config, hourLogStatus)

			// fetch logger
			go func(out chan<- string, inpp <-chan Status) {
				defer wg.Done()
				status := <-inpp
				if !status.Logged {
					out <- status.LogStatusMessage
					return
				}
				out <- fmt.Sprintf("%.2f h has been uploaded to %s", status.Current, status.Card)
			}(finalMessage, hourLogStatus)
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
}

// Upload the hour log.
func uploadHourLog(card string, minutes int, startTime int, config Config) (string, error) {
	url := config.Baseurl + "/issue/" + card + "/worklog"
	json, err := preparePayload(minutes, startTime)
	if err != nil {
		return "", fmt.Errorf("error preparing payload: %v", err)
	}

	status, err := makeRequest(url, json, config.Username, config.Key)
	if err != nil {
		return status, fmt.Errorf("error making request: %v", err)
	}

	return status, nil

}

// Prepare Payload to sent as part of the request.
func preparePayload(minutes int, startTime int) ([]byte, error) {
	location, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load time location Asia/Kolkata: %v", err)
		location = time.UTC
	}

	now := time.Now()
	formattedTime := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		startTime/100,
		startTime%100,
		0,
		0,
		location,
	).Format("2006-01-02T15:04:05.000-0700") // need something like "2021-01-17T12:34:00.000+0000"

	data, err := json.Marshal(Payload{Started: formattedTime, TimeSpentSeconds: minutes * 60})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return data, nil
}

// makeRequest makes the request to the API.
func makeRequest(url string, payload []byte, username string, key string) (string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request object: %v", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(username, key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to post data: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.Status, fmt.Errorf("failed to read body: %v", err)
	}

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return resp.Status, fmt.Errorf("not successful (%d): %v", resp.StatusCode, string(bodyBytes))
	}

	return resp.Status, nil
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
