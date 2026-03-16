package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/msjurset/runbook/internal/runbook"
)

func sendSlack(cfg *runbook.SlackConfig, subject, body string) error {
	webhook, err := resolveOpRef(cfg.Webhook, "slack_webhook")
	if err != nil {
		return fmt.Errorf("resolving webhook: %w", err)
	}

	payload := map[string]interface{}{
		"text": subject,
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*%s*", subject),
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": "```\n" + body + "```",
				},
			},
		},
	}

	if cfg.Channel != "" {
		payload["channel"] = cfg.Channel
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhook, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("posting to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack returned %d", resp.StatusCode)
	}
	return nil
}
