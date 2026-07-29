package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"infraalign/backend/internal/drift"
)

type SlackPayload struct {
	Text string `json:"text"`
}

func SendAlert(d drift.Difference) error {

	// Get the Slack webhook URL from environment variable
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		return fmt.Errorf("Slack webhook URL is not set")
	}

	//Build the message text
	message := fmt.Sprintf("Drift detected in resource: \nBucket: %s\nStatus: %s\nAttribute: %s\nExpected: %s\nActual: %s",d.BucketName, d.Status, d.AttributeName, d.ExpectedVal, d.ActualVal)

	//Create the payload struct and marshal to json
	payload := SlackPayload{Text: message}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	//Send HTTP Post request to slack
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Failed to send slack alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("slack alert failed with status: %d", resp.StatusCode)
	}

	return nil
 
}