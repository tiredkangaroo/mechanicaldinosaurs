package main

import (
	"context"
	"log/slog"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

var machineInfoRefreshAlerts = &Alerts[map[string]map[string]any]{}

func performDataRefresh() {
	var machineInfos = make(map[string]map[string]any)
	ms, err := getMachines()
	if err != nil {
		slog.Error("failed to get machines from database", "error", err)
		return
	}
	for _, machine := range ms {
		info, err := server.Call(server.GetInfoRequest, machine.Hostport, struct{}{}, machine.Secret)
		if err != nil {
			slog.Error("get machine info", "machine", machine.Name, "hostport", machine.Hostport, "error", err)
			machineInfos[machine.Name] = nil
		} else {
			machineInfos[machine.Name] = info.Map()
		}
	}
	slog.Info("data refresh", "machines_info", machineInfos)
	machineInfoRefreshAlerts.Broadcast(machineInfos)
}

func getMachines() ([]Machine, error) {
	rows, err := conn.Query(context.Background(), "SELECT name, hostport, secret_key FROM console_machine;")
	if err != nil {
		slog.Error("failed to query machines from database", "error", err)
		return nil, err
	}
	defer rows.Close()

	ms := []Machine{}
	for rows.Next() {
		var m Machine
		if err := rows.Scan(&m.Name, &m.Hostport, &m.Secret); err != nil {
			slog.Error("failed to scan machine from database", "error", err)
			continue
		}
		ms = append(ms, m)
	}

	return ms, nil
}

type Machine struct {
	Name     string `json:"name"`
	Hostport string `json:"hostport"` // hostport for the daemon
	Secret   string `json:"secret"`
}
