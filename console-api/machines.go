package main

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	database "github.com/tiredkangaroo/mechanicaldinosaurs/console-api/db"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

func addMachineRoutes(api *echo.Group, db *database.Queries) {
	api.GET("/api/machines", func(c echo.Context) error {
		machines, err := db.ListRemoteServers(c.Request().Context())
		if err != nil {
			slog.Error("list machines", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		var resp []MachineWithStatus
		for _, m := range machines {
			var mstatus MachineWithStatus
			mstatus.Name = m.Name
			mstatus.Hostport = m.Hostport

			info, err := server.Call(server.GetInfoRequest, m.Hostport, server.NoRequestData, m.Secret)
			if err != nil {
				slog.Error("get machine info", "name", m.Name, "error", err)
			} else {
				mstatus.Status = info
			}
			resp = append(resp, mstatus)
		}
		return c.JSON(200, resp)
	})

	api.GET("/api/machines/:name", func(c echo.Context) error {
		name := c.Param("name")
		machine, err := db.GetRemoteServer(c.Request().Context(), name)
		if err != nil {
			slog.Error("get machine", "name", name, "error", err)
			return c.JSON(404, map[string]string{"error": "machine not found"})
		}
		var resp MachineWithStatus
		resp.Name = machine.Name
		resp.Hostport = machine.Hostport

		info, err := server.Call(server.GetInfoRequest, machine.Hostport, server.NoRequestData, machine.Secret)
		if err != nil {
			slog.Error("get machine info", "name", machine.Name, "error", err)
		} else {
			resp.Status = info
		}

		return c.JSON(200, resp)
	})

	api.POST("/api/machines", func(c echo.Context) error {
		var remoteServer database.AddRemoteServerParams
		if err := c.Bind(&remoteServer); err != nil {
			slog.Error("bind request body", "error", err)
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if remoteServer.Name == "" || remoteServer.Hostport == "" || remoteServer.Secret == "" {
			return c.JSON(400, map[string]string{"error": "name, hostport, and secret are required"})
		}
		err := db.AddRemoteServer(c.Request().Context(), remoteServer)
		if err != nil {
			slog.Error("create machine", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		return c.JSON(201, nil)
	})
}
