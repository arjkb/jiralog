package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
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

// parseTasks reads the file specified by the filename, and returns a map of tasks
func parseTasks(config Config, filename string, apiUrl *url.URL) (map[string]Task, error) {
	tasks := make(map[string]Task)

	data, err := os.ReadFile(filename)
	if err != nil {
		// log.Fatal(err)
		return tasks, fmt.Errorf("failed to read input file: %v", err)
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
			// log.Fatal(err)
			return tasks, fmt.Errorf("failed to parse tasks: %v", err)
		}

		endTime, err := readTime(lines[i+1])
		if err != nil {
			// log.Fatal(err)
			return tasks, fmt.Errorf("failed to parse tasks: %v", err)
		}

		currDuration, err := computeDuration(startTime, endTime)
		if err != nil {
			// log.Fatal(err)
			return tasks, fmt.Errorf("failed to parse tasks: %v", err)
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

	return tasks, nil
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
	if len(line) < 4 {
		return 0, fmt.Errorf("improper time read: %q", line)
	}

	num, err := strconv.Atoi(line[:4])
	if err != nil {
		return 0, fmt.Errorf("failed to parse time to int: %v", err)
	}
	if num < 0 {
		return 0, fmt.Errorf("negative time read: %d", num)
	}

	minute := num % 100
	if minute > 59 {
		return 0, fmt.Errorf("minute greater than 59: %d", minute)
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

// computeDuration computes the difference
// between start and end times in minutes.
func computeDuration(start int, end int) (int, error) {
	const maxPossibleTime = 2359
	var minsTillHour int = 0

	if start > end {
		return 0, fmt.Errorf("end-time %d must be at least equal to start-time %d", end, start)
	}

	if start > maxPossibleTime || start < 0 {
		return 0, fmt.Errorf("start-time %d is invalid", start)
	}

	if end > maxPossibleTime || end < 0 {
		return 0, fmt.Errorf("end-time %d is invalid", end)
	}

	startMinutes, endMinutes := start%100, end%100
	if startMinutes > 59 {
		return 0, fmt.Errorf("start-time %d has invalid minutes of %d", start, startMinutes)
	}

	if endMinutes > 59 {
		return 0, fmt.Errorf("end-time %d has invalid minutes of %d", end, endMinutes)
	}

	hoursInBetween := (end / 100) - (start / 100)
	if startMinutes != 0 {
		hoursInBetween--
		minsTillHour = 60 - startMinutes
	}

	return minsTillHour + hoursInBetween*60 + endMinutes, nil
}
