package main

import (
	"log/slog"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

var machineInfoRefreshAlerts = &Alerts[map[string]*server.Info]{}
var machines = []Machine{}

func performDataRefresh() {
	var machineInfos = make(map[string]*server.Info)
	for _, machine := range machines {
		info, err := server.Call(server.GetInfoRequest, machine.Hostport, struct{}{}, machine.Secret)
		if err != nil {
			slog.Error("get machine info", "machine", machine.Name, "hostport", machine.Hostport, "error", err)
			machineInfos[machine.Name] = nil
		} else {
			machineInfos[machine.Name] = info
		}
	}
	slog.Info("data refresh", "machines_info", machineInfos)
	machineInfoRefreshAlerts.Broadcast(machineInfos)
}

type Machine struct {
	Name     string `json:"name"`
	Hostport string `json:"hostport"` // hostport for the daemon
	Secret   string `json:"secret"`
}
