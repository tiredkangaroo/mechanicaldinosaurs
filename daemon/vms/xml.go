package vms

import "encoding/xml"

// note: the *struct{} are for xml elements w no content, e.g. <acpi/>

type Domain struct {
	XMLName       xml.Name `xml:"domain"`
	Type          string   `xml:"type,attr"`
	Name          string   `xml:"name"`
	Memory        Memory   `xml:"memory"`
	CurrentMemory Memory   `xml:"currentMemory"`
	VCPU          VCPU     `xml:"vcpu"`
	OS            OS       `xml:"os"`
	Features      Features `xml:"features"`
	CPU           CPU      `xml:"cpu"`
	Clock         Clock    `xml:"clock"`
	OnPoweroff    string   `xml:"on_poweroff"`
	OnReboot      string   `xml:"on_reboot"`
	OnCrash       string   `xml:"on_crash"`
	Devices       Devices  `xml:"devices"`
}

type Memory struct {
	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

type VCPU struct {
	Placement string `xml:"placement,attr"`
	Value     int    `xml:",chardata"`
}

type OS struct {
	Type     OSType   `xml:"type"`
	Boot     []Boot   `xml:"boot"`
	BootMenu BootMenu `xml:"bootmenu"`
}

type OSType struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Value   string `xml:",chardata"`
}

type Boot struct {
	Dev string `xml:"dev,attr"`
}

type BootMenu struct {
	Enable string `xml:"enable,attr"`
}

type Features struct {
	ACPI *struct{} `xml:"acpi"`
	APIC *struct{} `xml:"apic"`
}

type CPU struct {
	Mode string `xml:"mode,attr"`
}

type Clock struct {
	Offset string  `xml:"offset,attr"`
	Timers []Timer `xml:"timer"`
}

type Timer struct {
	Name       string `xml:"name,attr"`
	TickPolicy string `xml:"tickpolicy,attr,omitempty"`
	Present    string `xml:"present,attr,omitempty"`
}

type Devices struct {
	Disks      []Disk      `xml:"disk"`
	Interfaces []Interface `xml:"interface"`
	Graphics   []Graphics  `xml:"graphics"`
	Video      Video       `xml:"video"`
	Inputs     []Input     `xml:"input"`
	Channels   []Channel   `xml:"channel"`
	MemBalloon MemBalloon  `xml:"memballoon"`
}

type Disk struct {
	Type     string     `xml:"type,attr"`
	Device   string     `xml:"device,attr"`
	Driver   DiskDriver `xml:"driver"`
	Source   DiskSource `xml:"source"`
	Target   DiskTarget `xml:"target"`
	ReadOnly *struct{}  `xml:"readonly"`
}

type DiskDriver struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Cache string `xml:"cache,attr,omitempty"`
}

type DiskSource struct {
	File string `xml:"file,attr"`
}

type DiskTarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type Interface struct {
	Type   string          `xml:"type,attr"`
	Source InterfaceSource `xml:"source"`
	Model  InterfaceModel  `xml:"model"`
}

type InterfaceSource struct {
	Network string `xml:"network,attr"`
}

type InterfaceModel struct {
	Type string `xml:"type,attr"`
}

type Graphics struct {
	Type     string          `xml:"type,attr"`
	Port     int             `xml:"port,attr"`
	AutoPort string          `xml:"autoport,attr"`
	Listen   string          `xml:"listen,attr"`
	Listens  []GraphicListen `xml:"listen"`
}

type GraphicListen struct {
	Type    string `xml:"type,attr"`
	Address string `xml:"address,attr"`
}

type Video struct {
	Model VideoModel `xml:"model"`
}

type VideoModel struct {
	Type string `xml:"type,attr"`
	VRam int    `xml:"vram,attr"`
}

type Input struct {
	Type string `xml:"type,attr"`
	Bus  string `xml:"bus,attr"`
}

type Channel struct {
	Type   string        `xml:"type,attr"`
	Target ChannelTarget `xml:"target"`
}

type ChannelTarget struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`
}

type MemBalloon struct {
	Model string `xml:"model,attr"`
}
