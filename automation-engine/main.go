package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

var AUTOMATIONS_SAVE_PATH = os.Getenv("AUTOMATIONS_SAVE_PATH")

func main() {
	loadSave()
	serve()
}

func loadSave() {
	automations = []*Automation{} // reset automations to empty slice before loading from file
	raw_data, err := os.ReadFile(AUTOMATIONS_SAVE_PATH)
	if err != nil {
		slog.Error("failed to read automations file", "error", err)
		return
	}
	var data struct {
		AutomationsCommunicable []*AutomationCommunicable `json:"automations"`
		Machines                []Machine                 `json:"machines"`
	}
	err = json.Unmarshal(raw_data, &data)
	if err != nil {
		slog.Error("failed to unmarshal automations file", "error", err)
		return
	}
	fmt.Println("loaded machines communicated from save file: ", len(data.Machines))
	fmt.Println("loaded automations communicated from save file: ", len(data.AutomationsCommunicable))

	machines = data.Machines

	for _, ac := range data.AutomationsCommunicable {
		automation, err := ac.ToAutomation()
		if err != nil {
			slog.Error("failed to convert communicated automation to automation", "error", err)
			continue
		}
		automations = append(automations, automation)

		if automation.Enabled { // this automation is enabled, so we should register its trigger
			err = automation.Trigger.Register(automation.OnTriggered)
			if err != nil {
				slog.Error("failed to register trigger for automation (disabling it now)", "automation_id", automation.ID, "error", err)
			}
			automation.Enabled = err == nil // if registration failed, we should mark the automation as disabled
		}
	}
}

func pushToSave() {
	data, err := json.MarshalIndent(struct {
		Automations []*Automation `json:"automations"`
		Machines    []Machine     `json:"machines"`
	}{
		Automations: automations,
		Machines:    machines,
	}, "", "  ") // again the marshal json function should handle the automations to make it communicable
	if err != nil {
		slog.Error("failed to marshal automations to save file", "error", err)
		return
	}
	err = os.WriteFile(AUTOMATIONS_SAVE_PATH, data, 0644)
	if err != nil {
		slog.Error("failed to write automations to save file", "error", err)
		return
	}
}
