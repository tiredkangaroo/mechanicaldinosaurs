package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/resend/resend-go/v2"
)

var resendClient = resend.NewClient(os.Getenv("RESEND_API_KEY"))
var emailDomain = os.Getenv("EMAIL_DOMAIN")
var slackBotToken = os.Getenv("SLACK_BOT_TOKEN")

type Action interface {
	Type() string
	Name() string
	Do(Context) error
}

// note: add email templating or something
type EmailAction struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (e *EmailAction) Type() string {
	return "email"
}
func (e *EmailAction) Name() string {
	return "send email to " + e.To
}

func (e *EmailAction) Do(ctx Context) error {
	params := &resend.SendEmailRequest{
		From:    "Infrastructure Automation Engine <infrastructure@" + emailDomain + ">",
		To:      []string{e.To},
		Subject: e.Subject,
		Text:    renderTemplate(e.Body, ctx),
	}
	_, err := resendClient.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send email: %v", err)
	} else {
		return nil
	}
}

type SlackAction struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

func (s *SlackAction) Type() string {
	return "slack"
}
func (s *SlackAction) Name() string {
	return "send slack message to " + s.ConversationID
}

func (s *SlackAction) Do(ctx Context) error {
	messageData := map[string]string{
		"channel": s.ConversationID,
		"text":    renderTemplate(s.Message, ctx),
	}
	messageDataJSON, err := json.Marshal(messageData)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message data: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(messageDataJSON))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+slackBotToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack request failed with status: %v", resp.Status)
	}

	return nil
}

// i asked ai for this regex lmao
var varRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

func renderTemplate(body string, ctx Context) string {
	matches := varRegex.FindAllStringSubmatch(body, -1)

	for _, match := range matches {
		varName := match[1]

		val, ok := ctx.Get(varName)
		if !ok {
			slog.Warn("variable not found in context", "variable_name", varName)
			continue
		}

		// match[0] is the full match {{ variable_name }}
		body = strings.ReplaceAll(body, match[0], fmt.Sprintf("%v", val))
	}

	return body
}

type ActionCommunicable struct {
	Type  string       `json:"type"`            // email, slack
	Email *EmailAction `json:"email,omitempty"` // exists for email action (wow who would've guessed lmao)
	Slack *SlackAction `json:"slack,omitempty"` // exists for slack action :explodes:
}
