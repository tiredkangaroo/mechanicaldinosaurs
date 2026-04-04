package vms

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	"libvirt.org/go/libvirt"
)

var MAX_MEMORY_MiB = uint(32768) // NOTE: this should be adjusted to something reasonable for the host machine
var MAX_DISK_GiB = uint(500)     // NOTE: see above
var MAX_VCPU = uint(runtime.NumCPU())
var dataDir = os.Getenv("MECHANICAL_DINOSAURS_DATA")
var alphanumericRegexp = regexp.MustCompile(`^[a-zA-Z0-9-.]*$`) // NOTE: check this regexp i lowk bs'd it

func ListVMs() ([]server.VM, error) {
	var vms []server.VM
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, fmt.Errorf("connect to hypervisor: %w", err)
	}
	defer conn.Close()

	domains, err := conn.ListAllDomains(0)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			slog.Error("get domain name", "error", err)
			continue
		}
		status, err := GetVMStatus(name)
		if err != nil {
			slog.Error("get VM status", "name", name, "error", err)
			continue
		}
		xmlDesc, err := domain.GetXMLDesc(0)
		if err != nil {
			slog.Error("get domain XML description", "name", name, "error", err)
			continue
		}
		cfg, err := getConfigFromXML(xmlDesc)
		if err != nil {
			slog.Error("get config from XML", "name", name, "error", err)
			continue
		}
		vms = append(vms, server.VM{
			Config: cfg,
			Status: status,
		})
	}
	return vms, nil
}

func getConfigFromXML(xmlDesc string) (server.VMConfig, error) {
	var d Domain
	if err := xml.Unmarshal([]byte(xmlDesc), &d); err != nil {
		return server.VMConfig{}, fmt.Errorf("unmarshal XML: %w", err)
	}
	// check memory unit is MiB
	if d.Memory.Unit != "MiB" {
		return server.VMConfig{}, fmt.Errorf("unexpected memory unit in domain XML: %s", d.Memory.Unit)
	}
	var bootISO Disk
	var primaryDisk Disk
	for _, disk := range d.Devices.Disks {
		if disk.Type == "file" && disk.Device == "cdrom" {
			bootISO = disk
		} else if disk.Type == "file" && disk.Device == "disk" {
			primaryDisk = disk
		}
	}
	if bootISO.Source.File == "" || primaryDisk.Source.File == "" {
		return server.VMConfig{}, fmt.Errorf("unexpected domain XML: missing boot ISO or primary disk")
	}
	return server.VMConfig{
		Name:      d.Name,
		VCPUs:     uint(d.VCPU.Value),
		MemoryMiB: uint(d.Memory.Value),
		BootFile:  filepath.Base(bootISO.Source.File),
	}, nil
}

// util functio nstuff

func dv(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func vncPortFromXML(xmlDesc string) (int, error) {
	var d struct {
		Devices struct {
			Graphics []struct {
				Type string `xml:"type,attr"`
				Port int    `xml:"port,attr"`
			} `xml:"graphics"`
		} `xml:"devices"`
	}
	if err := xml.Unmarshal([]byte(xmlDesc), &d); err != nil {
		return 0, err
	}
	for _, g := range d.Devices.Graphics {
		if g.Type == "vnc" && g.Port > 0 {
			return g.Port, nil
		}
	}
	return 0, fmt.Errorf("no vnc graphics found")
}
