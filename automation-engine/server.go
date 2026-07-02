package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

var AUTOMATION_ENGINE_PORT = os.Getenv("AUTOMATION_ENGINE_PORT")
var AUTOMATION_ENGINE_SECRET = os.Getenv("AUTOMATION_ENGINE_SECRET")

func serve() {
	srv := echo.New()
	api := srv.Group("/api")

	locked := false
	api.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if locked {
				slog.Info("request blocked due to lock", "path", c.Path(), "remote_addr", c.Request().RemoteAddr)
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			authorization := c.Request().Header.Get("Authorization")
			if authorization != "Bearer "+AUTOMATION_ENGINE_SECRET {
				slog.Info("request blocked due to invalid authorization", "path", c.Path(), "remote_addr", c.Request().RemoteAddr, "authorization", authorization)
				locked = true // permanently lock the server if an invalid authorization is received
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			return next(c)
		}
	})

	api.GET("/automations", func(c echo.Context) error {
		return c.JSON(200, automations) // the automation struct has a marshal json function to make it communicable
	})

	// add new automation
	api.POST("/automations", func(c echo.Context) error {
		var newAutomation Automation // there's an unmarshal json that handles this stuff, the actual json that should be given should be the communicable automation
		if err := c.Bind(&newAutomation); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if newAutomation.Enabled {
			err := newAutomation.Trigger.Register(newAutomation.OnTriggered)
			if err != nil {
				slog.Error("failed to register trigger for new automation (disabling it now)", "automation_id", newAutomation.ID, "error", err)
			}
			newAutomation.Enabled = err == nil // if registration failed, we should mark the automation as disabled
		}
		automations = append(automations, &newAutomation)
		slog.Info("new automation added", "automation_id", newAutomation.ID)
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// delete automation
	api.POST("/automations/delete", func(c echo.Context) error {
		var request struct {
			ID string `json:"id"`
		}
		if err := c.Bind(&request); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		var automation *Automation
		var index int
		for i, a := range automations {
			if a.ID == request.ID {
				automation = a
				index = i
				break
			}
		}
		if automation == nil {
			return c.JSON(404, map[string]string{"error": "automation not found"})
		}
		if err := automation.Disable(); err != nil {
			slog.Error("failed to disable automation", "automation_id", automation.ID, "error", err)
			return c.JSON(500, map[string]string{"error": "failed to disable automation: " + err.Error()})
		}
		automations = append(automations[:index], automations[index+1:]...) // remove the automation from the slice
		slog.Info("automation deleted", "automation_id", automation.ID)
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	api.POST("/automations/:id/enable", func(c echo.Context) error {
		id := c.Param("id")
		var automation *Automation
		for _, a := range automations {
			if a.ID == id {
				automation = a
				break
			}
		}
		if automation == nil {
			return c.JSON(404, map[string]string{"error": "automation not found"})
		}
		if err := automation.Enable(); err != nil {
			slog.Error("failed to enable automation", "automation_id", automation.ID, "error", err)
			return c.JSON(500, map[string]string{"error": "failed to enable automation: " + err.Error()})
		}
		slog.Info("automation enabled", "automation_id", automation.ID)
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	api.POST("/automations/:id/disable", func(c echo.Context) error {
		id := c.Param("id")
		var automation *Automation
		for _, a := range automations {
			if a.ID == id {
				automation = a
				break
			}
		}
		if automation == nil {
			return c.JSON(404, map[string]string{"error": "automation not found"})
		}
		if err := automation.Disable(); err != nil {
			slog.Error("failed to disable automation", "automation_id", automation.ID, "error", err)
			return c.JSON(500, map[string]string{"error": "failed to disable automation: " + err.Error()})
		}
		slog.Info("automation disabled", "automation_id", automation.ID)
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	api.POST("/machines", func(c echo.Context) error {
		machines = []Machine{}
		if err := c.Bind(&machines); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}

		slog.Info("machines updated", "machines", machines)
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// continuously refresh data every 30 seconds in a separate goroutine
	cancel := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				performDataRefresh()
			case <-cancel:
				return
			}
		}
	}()

	if err := srv.Start(":"); err != nil {
		slog.Error("server error", "error", err)
		cancel <- struct{}{} // wait for the data refresh goroutine to finish
	}
}
