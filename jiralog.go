// Program to log time entries to Jira.
// Author: Arjun Krishna Babu
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func main() {
	const PREFIX = "EXAMPLE" // TODO: make this configurable

	data, err := os.ReadFile("input.txt")
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines)-1; i++ {
		startTime, err := readTime(lines[i])
		if err != nil {
			log.Fatal(err)
		}

		endTime, err := readTime(lines[i+1])
		if err != nil {
			log.Fatal(err)
		}

		card, ok := readCard(lines[i], PREFIX)
		if !ok {
			// ignore lines that do not have card information
			continue
		}

		fmt.Printf("%d-%d %q: %s\n", startTime, endTime, card, lines[i])
	}
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
