package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/moby/moby/client"
	"github.com/tiredkangaroo/mechanicaldinosaurs/daemon/vms"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

var MECHANICAL_DINOSAURS_DATA = os.Getenv("MECHANICAL_DINOSAURS_DATA")
var API_SECRET = os.Getenv("API_SECRET")

func main() {
	e := echo.New()
	auth := Auth{Secret: API_SECRET}
	api := e.Group("", auth.Middleware)

	server.Handle(api, server.GetInfoRequest, func(c echo.Context, req struct{}) (*server.Info, error) {
		info, err := GetServerInfo()
		if err != nil {
			return nil, err
		}
		return info, nil
	})

	registerDockerRoutes(api)
	registerVMRoutes(api)

	slog.Info("starting server", "port", 6731)
	if err := e.Start(":6731"); err != nil {
		slog.Error("server error", "error", err)
	}
}

func registerDockerRoutes(api *echo.Group) {
	ds, err := NewDockerService()
	if err != nil {
		slog.Error("initialize docker service", "error", err)
	}
	available := err == nil

	dockerRouter := api.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !available {
				return c.JSON(503, map[string]string{"error": "docker functionality not available on this host"})
			}
			return next(c)
		}
	})

	server.Handle(dockerRouter, server.GetDockerAvailableRequest, func(c echo.Context, req struct{}) (*struct{}, error) {
		return nil, nil // middleware handles the response based on availability, so just return success here
	})

	server.Handle(dockerRouter, server.ListContainersRequest, func(c echo.Context, req struct{}) (*[]client.ContainerInspectResult, error) {
		containers, err := ds.ListContainers(c.Request().Context())
		if err != nil {
			return nil, err
		}
		return &containers, nil
	})

	server.Handle(dockerRouter, server.CreateContainerRequest, func(c echo.Context, req server.CreateContainerReq) (*struct{}, error) {
		_, err := ds.CreateContainer(c.Request().Context(), req.ContainerConfig)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(dockerRouter, server.StartContainerRequest, func(c echo.Context, req server.ContainerTargetReq) (*struct{}, error) {
		if err := ds.StartContainer(c.Request().Context(), req.ID); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(dockerRouter, server.StopContainerRequest, func(c echo.Context, req server.ContainerTargetReq) (*struct{}, error) {
		if err := ds.StopContainer(c.Request().Context(), req.ID, "SIGTERM"); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(dockerRouter, server.RemoveContainerRequest, func(c echo.Context, req server.ContainerTargetReq) (*struct{}, error) {
		if err := ds.RemoveContainer(c.Request().Context(), req.ID, true); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(dockerRouter, server.ComposeUpRequest, func(c echo.Context, req server.ComposeUpReq) (*struct{}, error) {
		if req.ComposeFilePath == "" {
			req.ComposeFilePath = filepath.Join(MECHANICAL_DINOSAURS_DATA, "docker_compose_files", req.ProjectName+".yaml")
			if err := os.WriteFile(req.ComposeFilePath, []byte(req.ComposeFileContent), 0644); err != nil {
				return nil, err
			}
		}
		if err := ds.ComposeUp(c.Request().Context(), req.ProjectName, req.ComposeFilePath); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(dockerRouter, server.ComposeDownRequest, func(c echo.Context, req server.ComposeDownReq) (*struct{}, error) {
		if err := ds.ComposeDown(c.Request().Context(), req.ProjectName); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func registerVMRoutes(api *echo.Group) {
	available, err := vms.Available()
	if err != nil {
		slog.Error("check VM availability", "error", err)
	}

	vmRouter := api.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !available {
				return c.JSON(503, map[string]string{"error": "vm functionality not available on this host"})
			}
			return next(c)
		}
	})

	server.Handle(vmRouter, server.GetVMAvailableRequest, func(c echo.Context, req struct{}) (*struct{}, error) {
		return nil, nil // middleware handles here
	})

	server.Handle(vmRouter, server.ListVMsRequest, func(c echo.Context, req struct{}) (*[]server.VM, error) {
		machines, err := vms.ListVMs()
		if err != nil {
			return nil, err
		}
		return &machines, nil
	})

	server.Handle(vmRouter, server.AvailableBootFilesRequest, func(c echo.Context, req struct{}) (*[]string, error) {
		entries, err := os.ReadDir(filepath.Join(MECHANICAL_DINOSAURS_DATA, "boot_files"))
		if err != nil {
			return nil, err
		}
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() {
				files = append(files, entry.Name())
			}
		}
		return &files, nil
	})

	server.Handle(vmRouter, server.CreateVMRequest, func(c echo.Context, req server.VMConfig) (*struct{}, error) {
		_, err := vms.CreateVM(&req)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.GetVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		_, err := vms.GetVM(req.Name)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.StartVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		if err := vms.StartVM(req.Name); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.StopVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		if err := vms.StopVM(req.Name, true); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.RestartVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		if err := vms.RestartVM(req.Name, true); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.UpdateVMRequest, func(c echo.Context, req server.UpdateVMReq) (*struct{}, error) {
		if err := vms.UpdateVM(req.Name, req.VCPUs, req.MemoryMiB, req.StorageGiB); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.DeleteVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		if err := vms.DeleteVM(req.Name); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
