package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
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

func (i Info) Map() map[string]any {
	return map[string]any{
		"os":               i.OS,
		"os_release":       i.OSRelease,
		"hostname":         i.Hostname,
		"uptime":           i.Uptime,
		"cpu":              i.CPU,
		"arch":             i.CPUArch,
		"cpu_num":          i.CPUNum,
		"cpu_usage":        i.CPUUsage,
		"cpu_temp":         i.CPUTemp,
		"memory":           i.MemoryCapacity,
		"memory_used":      i.MemoryUsed,
		"storage_capacity": i.StorageCapacity,
		"storage_used":     i.StorageUsed,
		"has_battery":      i.HasBattery,
		"battery":          i.Battery,
	}
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
	Config        VMConfig `json:"config"`
	Status        string   `json:"status"`
	MemoryUsedMiB uint     `json:"memory_used_mib"` // actual used memory in MiB
	DiskUsedGiB   uint     `json:"disk_used_gib"`   // actual used disk space in GiB
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

var GetVMRequest = StaticRequestInfo[struct{}, VM]{
	Method: "GET",
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

var DownloadISORequest = StaticRequestInfo[DownloadISOReq, struct{}]{
	Method: "GET",
	Path:   "/api/vms/download-iso",
}

type DownloadISOReq struct {
	URL     string `json:"url"`
	OSName  string `json:"os_name"` // e.g. "ubuntu-desktop" or "ubuntu-server"
	Version string `json:"version"` // e.g. "22.04" or "26.04"
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

var GetCloudflareTunnelTokenRequest = StaticRequestInfo[struct{}, []CloudflareTunnelIDorTokenResponse]{
	Method: "GET",
	Path:   "/api/tunnels/ids-and-tokens",
}

type CloudflareTunnelIDorTokenResponse struct {
	Type  string `json:"type"`  // "id" or "token"
	Value string `json:"value"` // the actual id or token
}

var GetPortsServicesRequest = StaticRequestInfo[struct{}, map[uint32]string]{ // port -> service name
	Method: "GET",
	Path:   "/api/ports-services",
}

var NoRequestData = struct{}{}
