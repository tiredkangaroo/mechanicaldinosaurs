// the notification package lowk manages notifcations
package notifications

import (
	"fmt"
	"log/slog"
)

// create the notifications "service" by just setting the methods struct as the type for a struct field in the config

type Methods struct {
	Slack *Slack `json:"slack"`
}

// Method interface
type Method interface {
	Notify(message string) error
}

// Notify sends out notifications with the message given to the methods given. it will only return an
// error there was an error using invoking a method of notification or no notifications could be send out.
func (m *Methods) Notify(methods []string, message string) error {
	var notifcationSent bool
	for _, methodName := range methods {
		var method Method
		switch methodName {
		case "slack":
			if m.Slack == nil {
				slog.Warn("slack not set up as a method of notification")
				continue
			}
			method = m.Slack
		default:
			slog.Warn("unsupported method of notification", "methodName", methodName)
			continue
		}
		if err := method.Notify(message); err != nil {
			return fmt.Errorf("error notifying using %s: %w", methodName, err)
		}
		notifcationSent = true
	}
	if !notifcationSent {
		return fmt.Errorf("no notifications were sent out")
	}
	return nil
}

func (m *Methods) Available() []string {
	var available []string
	if m.Slack != nil {
		available = append(available, "slack")
	}
	return available
}
