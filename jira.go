// This file contains the data structures helper methods
// for interacting with the Jira worklog API

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Payload struct {
	Comment          Comment `json:"comment"`
	Started          string  `json:"started"`
	TimeSpentSeconds int     `json:"timeSpentSeconds"`
}

type Comment struct {
	Content []Paragraph `json:"content"`
	Type    string      `json:"type"`
	Version int         `json:"version"`
}

type Paragraph struct {
	Content []TextNode `json:"content"`
	Type    string     `json:"type"`
}

type TextNode struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type GetWorklogResponse struct {
	Worklogs []struct {
		TimeSpentSeconds int `json:"timeSpentSeconds"`
	} `json:"worklogs"`
}

// Upload the hour log.
func uploadHourLog(card string, minutes int, startTime int, description string, config Config, baseUrl *url.URL) (string, error) {
	json, err := preparePayload(minutes, startTime, description)
	if err != nil {
		return "", fmt.Errorf("error preparing payload: %v", err)
	}

	resp, err := makeRequest(
		http.MethodPost,
		baseUrl.JoinPath("issue", card, "worklog").String(),
		json,
		config.Username,
		config.Key,
	)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
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

// Prepare Payload to sent as part of the request.
func preparePayload(minutes int, startTime int, description string) ([]byte, error) {
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

	data, err := json.MarshalIndent(Payload{
		Started:          formattedTime,
		TimeSpentSeconds: minutes * 60,
		Comment: Comment{
			Content: []Paragraph{
				{
					Content: []TextNode{
						{
							Text: description,
							Type: "text",
						},
					},
					Type: "paragraph",
				},
			},
			Type:    "doc",
			Version: 1,
		},
	}, "", "    ")
	// fmt.Println(string(data))
	// prettyJson, err := json.MarshalIndent()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return data, nil
}

// Stub method to get the spent-time for the card
func getTimeSpent(card string, config Config) (int, error) {
	url := config.Baseurl + "/issue/" + card + "/worklog"
	resp, err := makeRequest(http.MethodGet, url, nil, config.Username, config.Key)
	if err != nil {
		return 0, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read body: %v", err)
	}

	var response GetWorklogResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("JSON unmarshalling failed: %s", err)
	}

	totalSeconds := 0
	for _, worklog := range response.Worklogs {
		totalSeconds += worklog.TimeSpentSeconds
	}

	return totalSeconds, nil
}
