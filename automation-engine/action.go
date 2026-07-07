package main

import (
	"fmt"
	"os"

	"github.com/resend/resend-go/v2"
)

var resendClient = resend.NewClient(os.Getenv("RESEND_API_KEY"))
var emailDomain = os.Getenv("EMAIL_DOMAIN")

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
		Text:    e.Body,
	}
	_, err := resendClient.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send email: %v", err)
	} else {
		return nil
	}
}

// note: add slack action

type ActionCommunicable struct {
	Type  string       `json:"type"`            // email, slack
	Email *EmailAction `json:"email,omitempty"` // exists for email action (wow who would've guessed lmao)
}
