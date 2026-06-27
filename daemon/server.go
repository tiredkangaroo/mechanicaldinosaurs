package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/moby/moby/client"
	"github.com/tiredkangaroo/mechanicaldinosaurs/daemon/vms"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

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

	slog.Info("starting server", "port", PORT)
	if err := e.Start(":" + PORT); err != nil {
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

	server.Handle(dockerRouter, server.GetContainerRequest, func(c echo.Context, req struct{}) (*client.ContainerInspectResult, error) {
		id := c.QueryParam("id")
		if id == "" {
			return nil, echo.NewHTTPError(400, "missing query parameter: id")
		}
		res, err := ds.GetContainer(c.Request().Context(), id)
		return &res, err
	})

	dockerRouter.GET("/api/containers/:id/logs", func(c echo.Context) error {
		id := c.Param("id")
		pipe, err := ds.ContainerLogs(c.Request().Context(), id)
		if err != nil {
			return echo.NewHTTPError(500, "failed to get container logs: "+err.Error())
		}
		defer pipe.Close()
		// although c.Stream accepts an io.Reader, the pipe will still be closed when the context gets canceled (per the docs)
		return c.Stream(http.StatusOK, "text/plain", pipe)
	})

	server.Handle(dockerRouter, server.ListContainersRequest, func(c echo.Context, req struct{}) (*[]client.ContainerInspectResult, error) {
		containers, err := ds.ListContainers(c.Request().Context())
		if err != nil {
			return nil, err
		}
		return &containers, nil
	})

	server.Handle(dockerRouter, server.CreateContainerRequest, func(c echo.Context, req server.ContainerConfig) (*struct{}, error) {
		_, err := ds.CreateContainer(c.Request().Context(), req)
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

	server.Handle(vmRouter, server.GetVMRequest, func(c echo.Context, req struct{}) (*server.VM, error) {
		name := c.QueryParam("name")
		if name == "" {
			return nil, echo.NewHTTPError(400, "missing query parameter: name")
		}
		vm, err := vms.GetVM(name)
		return &vm, err
	})

	server.Handle(vmRouter, server.StartVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		if err := vms.StartVM(req.Name); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.StopVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		forcefulStr := c.QueryParam("force")
		forceful := forcefulStr == "true"
		if err := vms.StopVM(req.Name, !forceful); err != nil {
			return nil, err
		}
		return nil, nil
	})

	server.Handle(vmRouter, server.RestartVMRequest, func(c echo.Context, req server.VMTargetReq) (*struct{}, error) {
		forcefulStr := c.QueryParam("force")
		forceful := forcefulStr == "true"
		if err := vms.RestartVM(req.Name, !forceful); err != nil {
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

	vmRouter.GET("/api/vms/:name/proxy", func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return echo.NewHTTPError(400, "missing path parameter: name")
		}
		conn, _, err := c.Response().Hijack()
		if err != nil {
			return echo.NewHTTPError(500, "failed to hijack connection")
		}
		defer conn.Close()
		return vms.ProxyVM(name, conn)
	})
}
