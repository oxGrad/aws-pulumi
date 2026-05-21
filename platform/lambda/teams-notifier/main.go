package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type cloudWatchAlarm struct {
	AlarmName        string `json:"AlarmName"`
	AlarmDescription string `json:"AlarmDescription"`
	NewStateValue    string `json:"NewStateValue"`
	NewStateReason   string `json:"NewStateReason"`
	StateChangeTime  string `json:"StateChangeTime"`
	OldStateValue    string `json:"OldStateValue"`
}

type teamsCard struct {
	Type       string         `json:"@type"`
	Context    string         `json:"@context"`
	Summary    string         `json:"summary"`
	ThemeColor string         `json:"themeColor"`
	Title      string         `json:"title"`
	Sections   []teamsSection `json:"sections"`
}

type teamsSection struct {
	Facts []teamsFact `json:"facts"`
}

type teamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func handler(ctx context.Context, event events.SNSEvent) error {
	webhookURL := os.Getenv("TEAMS_WEBHOOK_URL")
	if webhookURL == "" {
		return fmt.Errorf("TEAMS_WEBHOOK_URL is not set")
	}

	for _, record := range event.Records {
		if err := notify(webhookURL, record.SNS); err != nil {
			return err
		}
	}
	return nil
}

func notify(webhookURL string, msg events.SNSEntity) error {
	var alarm cloudWatchAlarm
	if err := json.Unmarshal([]byte(msg.Message), &alarm); err != nil {
		alarm.AlarmName = msg.Subject
		alarm.NewStateValue = "UNKNOWN"
		alarm.NewStateReason = msg.Message
	}

	color := colorFor(alarm.NewStateValue)
	title := fmt.Sprintf("[%s] %s", alarm.NewStateValue, alarm.AlarmName)

	facts := []teamsFact{
		{Name: "State", Value: fmt.Sprintf("%s → %s", alarm.OldStateValue, alarm.NewStateValue)},
		{Name: "Reason", Value: alarm.NewStateReason},
		{Name: "Time", Value: alarm.StateChangeTime},
	}
	if alarm.AlarmDescription != "" {
		facts = append([]teamsFact{{Name: "Description", Value: alarm.AlarmDescription}}, facts...)
	}

	card := teamsCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		Summary:    alarm.AlarmName,
		ThemeColor: color,
		Title:      title,
		Sections:   []teamsSection{{Facts: facts}},
	}

	body, err := json.Marshal(card)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posting to Teams: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Teams webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func colorFor(state string) string {
	switch strings.ToUpper(state) {
	case "OK":
		return "00CC00"
	case "INSUFFICIENT_DATA":
		return "FFAA00"
	default: // ALARM
		return "FF0000"
	}
}

func main() {
	lambda.Start(handler)
}
