package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/tiredkangaroo/mechanicaldinosaurs/daemon/vms"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

var MECHANICAL_DINOSAURS_DATA = os.Getenv("MECHANICAL_DINOSAURS_DATA")

func main() {
	e := echo.New()
	api := e.Group("/api")

	api.GET("/info", func(c echo.Context) error {
		info, err := GetServerInfo()
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, info)
	})

	addVMRoutes(api)
	addDockerRoutes(api)
}

func addDockerRoutes(api *echo.Group) error {
	ds, err := NewDockerService()
	if err != nil {
		return err
	}
	api.GET("/containers", func(c echo.Context) error {
		containers, err := ds.ListContainers(c.Request().Context())
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, containers)
	})
	api.POST("/containers", func(c echo.Context) error {
		var req server.ContainerConfig
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		id, err := ds.CreateContainer(c.Request().Context(), req)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(201, map[string]any{
			"id":    id,
			"error": nil,
		})
	})
	api.POST("/container/:id/start", func(c echo.Context) error {
		id := c.Param("id")
		if err := ds.StartContainer(c.Request().Context(), id); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
	api.POST("/container/:id/stop", func(c echo.Context) error {
		id := c.Param("id")
		if err := ds.StopContainer(c.Request().Context(), id, "SIGTERM"); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
	api.DELETE("/container/:id", func(c echo.Context) error {
		id := c.Param("id")
		if err := ds.RemoveContainer(c.Request().Context(), id, true); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
	api.GET("/container/:id/logs", func(c echo.Context) error {
		id := c.Param("id")
		logs, err := ds.ContainerLogs(c.Request().Context(), id)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		// NOTE: closer??
		return c.Stream(200, "text/plain", logs)
	})
	api.POST("/compose/up", func(c echo.Context) error {
		var req struct {
			ProjectName        string `json:"projectName"`
			ComposeFileContent string `json:"composeFileContent"`
			ComposeFilePath    string `json:"composeFilePath"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if req.ComposeFilePath == "" {
			err := os.WriteFile(filepath.Join(MECHANICAL_DINOSAURS_DATA, "docker_compose_files", req.ProjectName+".yaml"), []byte(req.ComposeFileContent), 0644)
			if err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
			req.ComposeFilePath = filepath.Join(MECHANICAL_DINOSAURS_DATA, "docker_compose_files", req.ProjectName+".yaml")
		}
		if err := ds.ComposeUp(c.Request().Context(), req.ProjectName, req.ComposeFilePath); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
	// maybe this should be DELETE /compose/:name
	api.POST("/compose/down", func(c echo.Context) error {
		var req struct {
			ProjectName string `json:"projectName"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if err := ds.ComposeDown(c.Request().Context(), req.ProjectName); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
	return nil
}

func addVMRoutes(api *echo.Group) {
	var available bool
	var err error
	available, err = vms.Available()
	if err != nil {
		slog.Error("check VM availability", "error", err)
	}
	vmRouter := api.Group("/vms", func(next echo.HandlerFunc) echo.HandlerFunc {
		if !available {
			return func(c echo.Context) error {
				return c.JSON(503, map[string]string{"error": "vm functionality not available on this host"})
			}
		}
		return next
	})
	api.GET("/vms/available", func(c echo.Context) error {
		if available {
			return c.JSON(200, map[string]any{
				"available": true,
				"error":     nil,
			})
		} else {
			return c.JSON(200, map[string]any{
				"available": false,
				"error":     "vm functionality not available on this host",
			})
		}
	})
	vmRouter.GET("/vms", func(c echo.Context) error {
		machines, err := vms.ListVMs()
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, machines)
	})

	vmRouter.GET("/available-boot-files", func(c echo.Context) error {
		entries, err := os.ReadDir(filepath.Join(MECHANICAL_DINOSAURS_DATA, "boot_files"))
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		var files []string
		for _, entry := range entries {
			if entry.IsDir() { // shouldn't be any dirs in boot_files but just in case
				continue
			}
			files = append(files, entry.Name())
		}
		return c.JSON(200, files)
	})

	vmRouter.POST("/vms", func(c echo.Context) error {
		var config server.VMConfig
		if err := c.Bind(&config); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		_, err := vms.CreateVM(&config)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(201, map[string]any{
			"error": nil,
		})
	})

	vmRouter.GET("/vm/:name", func(c echo.Context) error {
		name := c.Param("name")
		vm, err := vms.GetVM(name)
		if err != nil {
			return c.JSON(404, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, vm)
	})

	vmRouter.POST("/vm/:name/start", func(c echo.Context) error {
		name := c.Param("name")
		if err := vms.StartVM(name); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})

	vmRouter.POST("/vm/:name/stop", func(c echo.Context) error {
		name := c.Param("name")
		if err := vms.StopVM(name, true); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})

	vmRouter.POST("/vm/:name/restart", func(c echo.Context) error {
		name := c.Param("name")
		if err := vms.RestartVM(name, true); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})

	vmRouter.PATCH("/vm/:name", func(c echo.Context) error {
		name := c.Param("name")
		var req struct {
			VCPUs      uint   `json:"vcpus,omitempty"`
			MemoryMiB  uint   `json:"memoryMiB,omitempty"`
			StorageGiB uint64 `json:"storageGiB,omitempty"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if err := vms.UpdateVM(name, req.VCPUs, req.MemoryMiB, req.StorageGiB); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})

	vmRouter.DELETE("/vm/:name", func(c echo.Context) error {
		name := c.Param("name")
		if err := vms.DeleteVM(name); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{
			"error": nil,
		})
	})
}
