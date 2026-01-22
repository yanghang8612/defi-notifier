package net

import (
	"time"

	"defi-notifier/config"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

type slackMessage struct {
	Text string `json:"text"`
}

var (
	client = resty.New().
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)
)

func ReportNotificationToSlack(msg string, isWarning bool) bool {
	if isWarning {
		msg += "\nPlease check this! <!channel>"
	}

	resp, err := client.R().SetBody(&slackMessage{Text: msg}).Post(config.C.Slack.Webhook)
	if err != nil {
		zap.S().Warnf("Failed to send message to Slack Channel: %s", err)
		return false
	}

	if resp.StatusCode() != 200 {
		zap.S().Warnf("Failed to send message to Slack Channel: status code %d, body: %s",
			resp.StatusCode(), resp.String())
		return false
	}

	return true
}
