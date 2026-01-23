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

func TestSlackWebhook(webhook string) bool {
	resp, err := client.R().Post(webhook)
	if err != nil {
		zap.S().Warnf("Failed to send test message to Slack Channel: %s", err)
		return false
	}

	if resp.StatusCode() != 400 || resp.String() != "invalid_payload" {
		zap.S().Warnf("Failed to send test message to Slack Channel: status code %d, body: %s",
			resp.StatusCode(), resp.String())
		return false
	}

	return true
}

func ReportToMainChannel(msg string, isWarning bool) bool {
	return reportToChannel(config.C.Slack.MainWebhook, msg, isWarning)
}

func ReportToBackupChannel(msg string, isWarning bool) bool {
	return reportToChannel(config.C.Slack.BackupWebhook, msg, isWarning)
}

func reportToChannel(webhook, msg string, isWarning bool) bool {
	var (
		resp *resty.Response
		err  error
	)

	if isWarning {
		resp, err = client.R().SetBody(&slackMessage{Text: msg + "\nPlease check this! <!channel>"}).Post(webhook)
	} else {
		resp, err = client.R().SetBody(&slackMessage{Text: msg}).Post(webhook)
	}

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
