package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DATABASE_URL = os.Getenv("DATABASE_URL")
var pool *pgxpool.Pool

func main() {
	defer pool.Close()
	if err := loadSave(); err != nil {
		slog.Error("failed to load automations from database", "error", err)
		return
	}
	slog.Info("loaded automations from database", "count", len(automations))
	serve()
}

func loadSave() error {
	automations = []*Automation{} // reset to empty slice; ts is a global var

	var err error
	pgxconfig, err := pgxpool.ParseConfig(DATABASE_URL)
	if err != nil {
		return fmt.Errorf("parse database URL: %w", err)
	}

	pool, err = pgxpool.NewWithConfig(context.Background(), pgxconfig)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	rows, err := pool.Query(context.Background(), "SELECT json_data FROM console_automation")
	if err != nil {
		return fmt.Errorf("query automations from database: %w", err)
	}
	defer rows.Close()

	var automationsCommunicable []AutomationCommunicable

	for rows.Next() {
		var d AutomationCommunicable
		if err := rows.Scan(&d); err != nil {
			slog.Error("failed to scan automation from database", "error", err)
		}
		automationsCommunicable = append(automationsCommunicable, d)
	}

	for _, ac := range automationsCommunicable {
		automation, err := ac.ToAutomation()
		if err != nil {
			slog.Error("failed to convert automation from communicable to internal representation", "automation_id", ac.ID, "error", err)
		}
		automations = append(automations, automation)

		if automation.Enabled { // this automation is enabled, so we should register its trigger
			err = automation.Trigger.Register(automation.OnTriggered)
			if err != nil {
				slog.Error("failed to register trigger for automation", "automation_id", automation.ID, "error", err)
			}
			automation.Enabled = err == nil // if registration failed, we should mark the automation as disabled
		}
	}
	return nil
}

func saveAutomation(automation *Automation) {
	ac := automation.ToCommunicable()
	pool.Exec(context.Background(), "INSERT INTO console_automation (automation_id, json_data) VALUES ($1, $2) ON CONFLICT (automation_id) DO UPDATE SET json_data = EXCLUDED.json_data", automation.ID, ac)
}
func deleteAutomation(automation *Automation) {
	pool.Exec(context.Background(), "DELETE FROM console_automation WHERE automation_id = $1", automation.ID)
}
