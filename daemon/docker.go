package main

import (
	"context"
	"io"

	docker "github.com/moby/moby/client"
)

type DockerService struct {
	client *docker.Client
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

func (ds *DockerService) CreateContainer(ctx context.Context, opts docker.ContainerCreateOptions) (string, error) {
	resp, err := ds.client.ContainerCreate(ctx, opts)
	return resp.ID, err
}

func (ds *DockerService) StartContainer(ctx context.Context, containerID string) error {
	_, err := ds.client.ContainerStart(ctx, containerID, docker.ContainerStartOptions{})
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
