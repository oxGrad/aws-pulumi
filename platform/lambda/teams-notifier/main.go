package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// webhookEntry holds per-service webhook URLs keyed by severity.
type webhookEntry struct {
	Critical string `json:"critical"`
	Warning  string `json:"warning"`
}

// webhookMap is the JSON structure stored in SSM:
//
//	{
//	  "app-sandbox": {"critical": "https://...", "warning": "https://..."},
//	  "app-travel":  {"critical": "https://...", "warning": "https://..."}
//	}
type webhookMap map[string]webhookEntry

var (
	cachedMap   webhookMap
	cacheMu     sync.RWMutex
	ssmClient   *ssm.Client
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config: %v", err))
	}
	ssmClient = ssm.NewFromConfig(cfg)
}

func getWebhookMap(ctx context.Context) (webhookMap, error) {
	cacheMu.RLock()
	if cachedMap != nil {
		m := cachedMap
		cacheMu.RUnlock()
		return m, nil
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()

	paramPath := os.Getenv("SSM_PARAMETER_PATH")
	out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramPath),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching SSM parameter %q: %w", paramPath, err)
	}

	var m webhookMap
	if err := json.Unmarshal([]byte(*out.Parameter.Value), &m); err != nil {
		return nil, fmt.Errorf("parsing webhook map: %w", err)
	}
	cachedMap = m
	return m, nil
}

// severityFromTopicARN returns "critical" or "warning" by matching the SNS topic ARN.
func severityFromTopicARN(topicARN string) string {
	if topicARN == os.Getenv("CRITICAL_TOPIC_ARN") {
		return "critical"
	}
	return "warning"
}

// webhookURLFor finds the webhook URL for the given alarm name and severity.
// It matches service names as substrings of the alarm name, preferring longer matches.
func webhookURLFor(wm webhookMap, alarmName, severity string) string {
	keys := make([]string, 0, len(wm))
	for k := range wm {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	for _, svc := range keys {
		if strings.Contains(alarmName, svc) {
			entry := wm[svc]
			if severity == "critical" {
				return entry.Critical
			}
			return entry.Warning
		}
	}
	return ""
}

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
	wm, err := getWebhookMap(ctx)
	if err != nil {
		return err
	}

	for _, record := range event.Records {
		severity := severityFromTopicARN(record.SNS.TopicArn)

		var alarm cloudWatchAlarm
		if err := json.Unmarshal([]byte(record.SNS.Message), &alarm); err != nil {
			alarm.AlarmName = record.SNS.Subject
			alarm.NewStateValue = "UNKNOWN"
			alarm.NewStateReason = record.SNS.Message
		}

		webhookURL := webhookURLFor(wm, alarm.AlarmName, severity)
		if webhookURL == "" {
			fmt.Printf("no webhook configured for alarm %q (severity: %s), skipping\n", alarm.AlarmName, severity)
			continue
		}

		if err := postToTeams(webhookURL, alarm); err != nil {
			return err
		}
	}
	return nil
}

func postToTeams(webhookURL string, alarm cloudWatchAlarm) error {
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
		ThemeColor: colorFor(alarm.NewStateValue),
		Title:      fmt.Sprintf("[%s] %s", alarm.NewStateValue, alarm.AlarmName),
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
	default:
		return "FF0000"
	}
}

func main() {
	lambda.Start(handler)
}
