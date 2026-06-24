package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/moby/moby/client"
)

type Info struct {
	// host info
	OS        string `json:"os"`
	OSRelease string `json:"os_release"`
	Hostname  string `json:"hostname"`
	Uptime    uint64 `json:"uptime"` // system uptime in seconds

	// cpu info
	CPU      string  `json:"cpu"`       // cpu model
	CPUArch  string  `json:"arch"`      // architecture
	CPUNum   int     `json:"cpu_num"`   // number of cpu cores
	CPUUsage float64 `json:"cpu_usage"` // cpu usage percentage
	CPUTemp  float64 `json:"cpu_temp"`  // cpu temperature in Celsius

	// memory info
	MemoryCapacity uint64 `json:"memory"`      // total memory in bytes
	MemoryUsed     uint64 `json:"memory_used"` // used memory in bytes

	// storage info
	StorageCapacity uint64 `json:"storage_capacity"` // total storage capacity in bytes
	StorageUsed     uint64 `json:"storage_used"`     // used storage in bytes

	// battery info
	HasBattery bool   `json:"has_battery"`       // whether the system has a battery
	Battery    string `json:"battery,omitempty"` // battery model
}

type Service struct {
	Name        string `json:"name"` // will be in form: name.service
	Description string `json:"description"`
	Status      string `json:"status"`
	Contents    string `json:"contents"` // full content of the service file
}

type VMConfig struct {
	Name          string `json:"name"`
	VCPUs         uint   `json:"vcpus"`
	MemoryMiB     uint   `json:"memory_mib"`
	BootFile      string `json:"boot_file"`      // $MECHANICAL_DINOSAURS_DATA/boot_files/<boot_file> should exist on the server
	DiskGiB       uint   `json:"disk_gib"`       // size of the primary qcow2 disk
	NetworkBridge string `json:"network_bridge"` // e.g. "virbr0" (default NAT bridge)
	GraphicsType  string `json:"graphics_type"`  // e.g. "spice" or "vnc"
}

type VM struct {
	Config VMConfig `json:"config"`
	Status string   `json:"status"`
}

type ContainerConfig struct {
	Name          string   `json:"name"`
	Image         string   `json:"image"`          // e.g. "nginx:latest"
	ExposedPorts  []string `json:"exposed_ports"`  // list of ports in form "80/tcp", "53/udp", etc.
	Env           []string `json:"env"`            // list of environment variables in form "KEY=value"
	Cmd           []string `json:"cmd"`            // command to run in the container on start
	Volumes       []string `json:"volumes"`        // list of volumes in form "/host/path:/container/path"
	RestartPolicy string   `json:"restart_policy"` // e.g. "no", "on-failure", "always", "unless-stopped"
	MaxRetryCount int      `json:"retry_count"`
	AutoRemove    bool     `json:"auto_remove"` // whether to automatically remove the container when it exits
}

func Call[Req any, Res any](reqinfo StaticRequestInfo[Req, Res], hostport string, req Req, authorization string) (*Res, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// NOTE: certs when??
	httpReq, err := http.NewRequest(reqinfo.Method, "http://"+hostport+reqinfo.Path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+authorization)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d and invalid error response", resp.StatusCode)
		}
		if errMessage, ok := errResp["error"]; ok {
			return nil, errors.New(errMessage)
		} else {
			return nil, fmt.Errorf("request failed with status %d and no error message", resp.StatusCode)
		}
	}

	var res Res
	if any(res) != any(struct{}{}) { // only try to decode if response is not empty struct
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}
	}
	return &res, nil
}

func Handle[Req any, Res any](group *echo.Group, reqinfo StaticRequestInfo[Req, Res], handler func(echo.Context, Req) (*Res, error)) {
	group.Add(reqinfo.Method, reqinfo.Path, func(c echo.Context) error {
		var req Req
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		res, err := handler(c, req)
		if err == nil {
			return c.JSON(200, res)
		}
		if errResp, ok := err.(ErrorResponse); ok {
			return c.JSON(errResp.Status, map[string]string{"error": errResp.Message})
		}
		return c.JSON(500, map[string]string{"error": err.Error()})
	})
}

// static request stuff (method & path) but also the request and response types for type safety in handlers and calls
type StaticRequestInfo[Req any, Res any] struct {
	Method string
	Path   string
}

// erroneous response.
type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"error"`
}

func (e ErrorResponse) Error() string {
	return e.Message
}

var GetInfoRequest = StaticRequestInfo[struct{}, Info]{
	Method: "GET",
	Path:   "/api/info",
}

// docker

var GetDockerAvailableRequest = StaticRequestInfo[struct{}, struct{}]{
	Method: "GET",
	Path:   "/api/docker/available",
}

var ListContainersRequest = StaticRequestInfo[struct{}, []client.ContainerInspectResult]{
	Method: "GET",
	Path:   "/api/containers/list",
}

type CreateContainerReq struct {
	ContainerConfig // Embedded from your existing server.ContainerConfig
}

var CreateContainerRequest = StaticRequestInfo[CreateContainerReq, struct{}]{
	Method: "POST",
	Path:   "/api/containers/create",
}

type ContainerTargetReq struct {
	ID string `json:"id"`
}

var StartContainerRequest = StaticRequestInfo[ContainerTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/containers/start",
}
var StopContainerRequest = StaticRequestInfo[ContainerTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/containers/stop",
}
var RemoveContainerRequest = StaticRequestInfo[ContainerTargetReq, struct{}]{
	Method: "POST", // Changed from DELETE since path parameters are removed
	Path:   "/api/containers/remove",
}

type ComposeUpReq struct {
	ProjectName        string `json:"projectName"`
	ComposeFileContent string `json:"composeFileContent"`
	ComposeFilePath    string `json:"composeFilePath"`
}

var ComposeUpRequest = StaticRequestInfo[ComposeUpReq, struct{}]{
	Method: "POST",
	Path:   "/api/compose/up",
}

type ComposeDownReq struct {
	ProjectName string `json:"projectName"`
}

var ComposeDownRequest = StaticRequestInfo[ComposeDownReq, struct{}]{
	Method: "POST",
	Path:   "/api/compose/down",
}

// vm

var GetVMAvailableRequest = StaticRequestInfo[struct{}, struct{}]{
	Method: "GET",
	Path:   "/api/vms/available",
}

var ListVMsRequest = StaticRequestInfo[struct{}, []VM]{
	Method: "GET",
	Path:   "/api/vms/list",
}

var AvailableBootFilesRequest = StaticRequestInfo[struct{}, []string]{
	Method: "GET",
	Path:   "/api/vms/boot-files",
}

var CreateVMRequest = StaticRequestInfo[VMConfig, struct{}]{
	Method: "POST",
	Path:   "/api/vms/create",
}

type VMTargetReq struct {
	Name string `json:"name"`
}

var GetVMRequest = StaticRequestInfo[VMTargetReq, struct{}]{
	Method: "POST", // uh we send the name in the body. hmm i wonder if this is stupid. anything but grpc tho right
	Path:   "/api/vms/get",
}
var StartVMRequest = StaticRequestInfo[VMTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/vms/start",
}
var StopVMRequest = StaticRequestInfo[VMTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/vms/stop",
}
var RestartVMRequest = StaticRequestInfo[VMTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/vms/restart",
}
var DeleteVMRequest = StaticRequestInfo[VMTargetReq, struct{}]{
	Method: "POST",
	Path:   "/api/vms/delete",
}

type UpdateVMReq struct {
	Name       string `json:"name"`
	VCPUs      uint   `json:"vcpus,omitempty"`
	MemoryMiB  uint   `json:"memoryMiB,omitempty"`
	StorageGiB uint64 `json:"storageGiB,omitempty"`
}

var UpdateVMRequest = StaticRequestInfo[UpdateVMReq, struct{}]{
	Method: "PATCH",
	Path:   "/api/vms/update",
}

var NoRequestData = struct{}{}
