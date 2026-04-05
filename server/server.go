package server

import "fmt"

type RemoteServer struct {
	Name     string `json:"name"`
	Hostport string `json:"hostport"`
	Secret   string `json:"secret"`
}

// adding the marshal function to avoid sending secret but also not using - in secret json tag
// bc we want unmarshal
func (s *RemoteServer) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"name":"%s","hostport":"%s"}`, s.Name, s.Hostport)), nil
}

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
