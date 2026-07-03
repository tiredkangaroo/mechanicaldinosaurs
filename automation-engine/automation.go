package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

var automations = []*Automation{}

type Automation struct {
	// automation consists of trigger, conditions, and actions
	ID        string
	Enabled   bool
	Trigger   Trigger
	Condition *Condition
	Action    Action
}

func (a *Automation) String() string {
	triggerName := a.Trigger.Name()
	actionName := a.Action.Name()

	s := "on event: " + triggerName
	if a.Condition != nil {
		s += ", if condition " + a.Condition.String() + " is met"
	}
	s += ", then do " + actionName
	return s
}

func (a *Automation) OnTriggered(c Context) {
	slog.Info("automation triggered", "automation_id", a.ID)

	var v bool = true
	var err error

	if a.Condition != nil {
		v, err = a.Condition.Evaluate(c)
		if err != nil {
			slog.Error("failed to evaluate condition", "automation_id", a.ID, "error", err)
		}
		slog.Info("automation condition evaluated", "automation_id", a.ID, "result", v)
	}
	if v { // if condition is met (or no condition), then do action
		err = a.Action.Do(c)
		slog.Info("automation action executed", "automation_id", a.ID, "error", err)
	}
}

func (a *Automation) Enable() error {
	if a.Enabled {
		return nil
	}
	err := a.Trigger.Register(a.OnTriggered)
	if err != nil {
		return fmt.Errorf("failed to register trigger: %w", err)
	}
	a.Enabled = true
	return nil
}

func (a *Automation) Disable() error {
	if !a.Enabled {
		return nil
	}
	err := a.Trigger.Deregister()
	if err != nil {
		return fmt.Errorf("failed to unregister trigger: %w", err)
	}
	a.Enabled = false
	return nil
}

func (a *Automation) MarshalJSON() ([]byte, error) {
	ac := a.ToCommunicable()
	return json.Marshal(ac)
}

func (a *Automation) UnmarshalJSON(data []byte) error {
	var ac AutomationCommunicable
	err := json.Unmarshal(data, &ac)
	if err != nil {
		return err
	}
	automation, err := ac.ToAutomation()
	if err != nil {
		return err
	}
	*a = *automation
	return nil
}

// version of automation that can be communicated (via json)
type AutomationCommunicable struct {
	ID        string              `json:"id"`
	Enabled   bool                `json:"enabled"`
	Trigger   TriggerCommunicable `json:"trigger"`
	Condition *Condition          `json:"condition,omitempty"`
	Action    ActionCommunicable  `json:"action"`
}

func (a *Automation) ToCommunicable() *AutomationCommunicable {
	ac := &AutomationCommunicable{
		ID:      a.ID,
		Enabled: a.Enabled,
		Trigger: TriggerCommunicable{
			Type: a.Trigger.Type(),
		},
		Condition: a.Condition,
		Action: ActionCommunicable{
			Type: a.Action.Type(),
		},
	}

	switch triggerTyped := a.Trigger.(type) {
	case *TimeTrigger:
		ac.Trigger.Time = triggerTyped.Time.Unix()
	case *IntervalTrigger:
		ac.Trigger.Every = triggerTyped.Every.Seconds()
	}

	switch a.Action.(type) {
	case *EmailAction:
		ac.Action.Email = a.Action.(*EmailAction)
	}

	return ac
}

// note: this function does not register the trigger
func (ac *AutomationCommunicable) ToAutomation() (*Automation, error) {
	var trigger Trigger
	switch ac.Trigger.Type {
	case "time":
		trigger = NewTimeTrigger(time.Unix(ac.Trigger.Time, 0))
	case "interval":
		trigger = NewIntervalTrigger(time.Duration(ac.Trigger.Every) * time.Second)
	case "machines info refresh":
		trigger = NewMachineInfoRefreshTrigger()
	default:
		return nil, fmt.Errorf("unknown trigger type: %s", ac.Trigger.Type)
	}
	var action Action
	switch ac.Action.Type {
	case "email":
		if ac.Action.Email == nil {
			return nil, fmt.Errorf("email action is nil but action type is email")
		}
		action = ac.Action.Email
	default:
		return nil, fmt.Errorf("unknown action type: %s", ac.Action.Type)
	}

	return &Automation{
		ID:        ac.ID,
		Enabled:   ac.Enabled,
		Trigger:   trigger,
		Condition: ac.Condition,
		Action:    action,
	}, nil
}

// // example automation
// func exampleAutomation() *Automation {
// 	trigger := NewMachineInfoRefreshTrigger()
// 	condition := Condition{
// 		Variable: "machines_info.pineapple.cpu_temp",
// 		Operator: OperatorGreaterThan,
// 		Value:    50,
// 	}
// 	action := &EmailAction{
// 		To:      "ajinest6@gmail.com",
// 		Subject: "CPU temperature alert",
// 		Body:    "The CPU temperature of pineapple has exceeded 50 degrees celsius.",
// 	}

// 	return &Automation{
// 		Trigger:   trigger,
// 		Condition: condition,
// 		Action:    action,
// 	}
// 	// on event: machines info refresh, if condition machines_info.pineapple.cpu_temp > 50 is met, then do send email to ajinest6@gmail.com
// }
