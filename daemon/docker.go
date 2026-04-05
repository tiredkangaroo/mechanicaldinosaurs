package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	docker "github.com/moby/moby/client"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

type DockerService struct {
	client         *docker.Client
	composeService api.Compose
}

func (ds *DockerService) PullImage(ctx context.Context, refStr string, registryAuth string) (io.ReadCloser, error) {
	resp, err := ds.client.ImagePull(ctx, refStr, docker.ImagePullOptions{
		RegistryAuth: registryAuth,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (ds *DockerService) ListImages(ctx context.Context) ([]string, error) {
	images, err := ds.client.ImageList(ctx, docker.ImageListOptions{})
	if err != nil {
		return nil, err
	}
	var imageRefs []string
	for _, image := range images.Items {
		imageRefs = append(imageRefs, image.ID)
	}
	return imageRefs, nil
}

func (ds *DockerService) RemoveImage(ctx context.Context, refStr string) error {
	_, err := ds.client.ImageRemove(ctx, refStr, docker.ImageRemoveOptions{})
	return err
}

func (ds *DockerService) ComposeUp(ctx context.Context, name, composeFilePath string) error {
	proj, err := ds.composeService.LoadProject(ctx, api.ProjectLoadOptions{
		ConfigPaths: []string{composeFilePath},
		ProjectName: name,
	})
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	return ds.composeService.Up(ctx, proj, api.UpOptions{
		Create: api.CreateOptions{},
		Start:  api.StartOptions{},
	})
}

func (ds *DockerService) ComposeDown(ctx context.Context, name string) error {
	return ds.composeService.Down(ctx, name, api.DownOptions{
		RemoveOrphans: true,
	})
}

func (ds *DockerService) CreateContainer(ctx context.Context, config server.ContainerConfig) (string, error) {
	exposedPorts := make(network.PortSet, len(config.ExposedPorts))
	for _, port := range config.ExposedPorts {
		portSplit := strings.Split(port, "/")
		if len(portSplit) != 2 {
			return "", fmt.Errorf("invalid port format: %s", port)
		}
		n, err := strconv.Atoi(portSplit[0])
		if err != nil {
			return "", fmt.Errorf("invalid port number: %s", portSplit[0])
		}
		if n < 1 || n > 65535 {
			return "", fmt.Errorf("port number out of range: %d", n)
		}
		p, ok := network.PortFrom(uint16(n), network.IPProtocol(portSplit[1]))
		if !ok {
			return "", fmt.Errorf("bad port: %s", port)
		}
		exposedPorts[p] = struct{}{}
	}
	resp, err := ds.client.ContainerCreate(ctx, docker.ContainerCreateOptions{
		Config: &container.Config{
			Image:        config.Image,
			Env:          config.Env,
			Cmd:          config.Cmd,
			ExposedPorts: exposedPorts,
			// idk how to do volumes here im ngl
		},
		HostConfig: &container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name:              container.RestartPolicyMode(config.RestartPolicy),
				MaximumRetryCount: config.MaxRetryCount,
			},
			AutoRemove: config.AutoRemove,
		},
	})
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (ds *DockerService) StartContainer(ctx context.Context, containerID string) error {
	_, err := ds.client.ContainerStart(ctx, containerID, docker.ContainerStartOptions{})
	return err
}

func (ds *DockerService) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	_, err := ds.client.ContainerRemove(ctx, containerID, docker.ContainerRemoveOptions{
		Force: force,
	})
	return err
}

func (ds *DockerService) StopContainer(ctx context.Context, containerID string, signal string) error {
	timeout := 10 // seconds
	_, err := ds.client.ContainerStop(ctx, containerID, docker.ContainerStopOptions{
		Signal:  signal,
		Timeout: &timeout,
	})
	return err
}

func (ds *DockerService) ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	pipe, err := ds.client.ContainerLogs(ctx, containerID, docker.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return nil, err
	}
	return pipe, nil
}

func (ds *DockerService) ListContainers(ctx context.Context) ([]docker.ContainerInspectResult, error) {
	containers, err := ds.client.ContainerList(ctx, docker.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	var is []docker.ContainerInspectResult
	for _, containerSummary := range containers.Items {
		result, err := ds.client.ContainerInspect(ctx, containerSummary.ID, docker.ContainerInspectOptions{})
		if err != nil {
			continue
		}
		is = append(is, result)
	}
	return is, nil
}

func NewDockerService() (*DockerService, error) {
	cli, err := docker.New(docker.FromEnv)
	if err != nil {
		return nil, err
	}
	dcli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("create Docker CLI: %w", err)
	}
	if err := dcli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, fmt.Errorf("initialize Docker CLI: %w", err)
	}
	composeService, err := compose.NewComposeService(dcli)
	if err != nil {
		return nil, fmt.Errorf("create Compose service: %w", err)
	}
	return &DockerService{client: cli, composeService: composeService}, nil
}
