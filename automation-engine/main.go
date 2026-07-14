package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
)

var DATABASE_URL = os.Getenv("DATABASE_URL")
var conn *pgx.Conn

func main() {
	loadSave()
	serve()
}

func loadSave() {
	automations = []*Automation{} // reset to empty slice; ts is a global var

	var err error
	conn, err = pgx.Connect(context.Background(), DATABASE_URL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return
	}
	rows, err := conn.Query(context.Background(), "SELECT json_data FROM console_automation")
	if err != nil {
		slog.Error("failed to query automations from database", "error", err)
		return
	}
	defer rows.Close()

	var automationsCommunicable []AutomationCommunicable

	for rows.Next() {
		var d AutomationCommunicable
		if err := rows.Scan(&d); err != nil {
			slog.Error("failed to scan automations from database", "error", err)
			continue
		}
		automationsCommunicable = append(automationsCommunicable, d)
	}

	for _, ac := range automationsCommunicable {
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

func saveAutomation(automation *Automation) {
	ac := automation.ToCommunicable()
	conn.Exec(context.Background(), "INSERT INTO console_automation (automation_id, json_data) VALUES ($1, $2) ON CONFLICT (automation_id) DO UPDATE SET json_data = EXCLUDED.json_data", automation.ID, ac)
}
func deleteAutomation(automation *Automation) {
	conn.Exec(context.Background(), "DELETE FROM console_automation WHERE automation_id = $1", automation.ID)
}
