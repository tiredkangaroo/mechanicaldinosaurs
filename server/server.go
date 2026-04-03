package server

type RemoteServer struct {
	Hostport string `json:"hostport"`
	Secret   string `json:"secret"`
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
	ISOName       string `json:"iso_name"`       // $MECHANICAL_DINOSAURS_DATA/isos/<iso_name>.iso should exist on the server
	DiskGiB       uint   `json:"disk_gib"`       // size of the primary qcow2 disk
	NetworkBridge string `json:"network_bridge"` // e.g. "virbr0" (default NAT bridge)
}
