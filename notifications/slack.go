package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Slack struct {
	ChannelID string `json:"channel_id"` // this can be a dm id as well
	Token     string `json:"token"`      // xoxb token, needs chat:write scope
}

func (s Slack) Notify(message string) error {
	data := map[string]string{
		"channel": s.ChannelID,
		"text":    message,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("slack marshal error: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack create req: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+s.Token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack do req: %w")
	}
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("slack read body: %w", err)
	}
	if resp.Status[0] != '2' {
		return fmt.Errorf("slack not 2xx status code: %s", string(respData))
	}
	return nil
}
