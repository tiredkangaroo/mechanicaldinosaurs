package vms

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	"libvirt.org/go/libvirt"
)

var MAX_MEMORY_MiB = uint(32768) // NOTE: this should be adjusted to something reasonable for the host machine
var MAX_DISK_GiB = uint(500)     // NOTE: see above
var MAX_VCPU = uint(runtime.NumCPU())
var dataDir = os.Getenv("MECHANICAL_DINOSAURS_DATA")
var alphanumericRegexp = regexp.MustCompile(`^[a-zA-Z0-9-.]*$`) // NOTE: check this regexp i lowk bs'd it

// available just returns if doing vms is possible.
func Available() (bool, error) {
	switch runtime.GOARCH {
	case "amd64":
		// since we're on intel/amd we can check /proc/cpuinfo for vmx or svm flags
		// for hardware virtualization support
		cpuInfo, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			return false, fmt.Errorf("read /proc/cpuinfo: %w", err)
		}
		// i'm not sure but there's a small chance this has false positives bc we're not actually getting the flags
		// field but this is good enough
		if !bytes.Contains(cpuInfo, []byte("vmx")) && !bytes.Contains(cpuInfo, []byte("svm")) {
			return false, fmt.Errorf("vmx or svm flags not found in /proc/cpuinfo, hardware virtualization support may not be available")
		}
	case "arm64":
	default:
		return false, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		return false, fmt.Errorf("kvm support not available")
	}

	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return false, fmt.Errorf("connect to hypervisor: %w", err)
	}
	conn.Close()

	requiredDrivers := []string{
		"virtio-win.iso", // for windows VMs to have virtio drivers available during installation
	}
	drivers := []string{}
	entries, err := os.ReadDir(filepath.Join(dataDir, "drivers"))
	if err != nil {
		return false, fmt.Errorf("read drivers directory: %w", err)
	}
	for _, entry := range entries {
		if err != nil {
			return false, fmt.Errorf("read drivers directory: %w", err)
		}
		if entry.IsDir() {
			continue
		}
		drivers = append(drivers, entry.Name())
	}
	for _, required := range requiredDrivers {
		if !slices.Contains(drivers, required) {
			return false, fmt.Errorf("required driver %s not found in drivers directory", required)
		}
	}

	return true, nil
}

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
		cfg, err := GetConfigFromXML(xmlDesc)
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

func GetVM(name string) (server.VM, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return server.VM{}, fmt.Errorf("connect to hypervisor: %w", err)
	}
	defer conn.Close()

	domain, err := conn.LookupDomainByName(name)
	if err != nil {
		return server.VM{}, fmt.Errorf("lookup domain: %w", err)
	}

	status, err := GetVMStatus(name)
	if err != nil {
		return server.VM{}, fmt.Errorf("get VM status: %w", err)
	}

	xmlDesc, err := domain.GetXMLDesc(0)
	if err != nil {
		return server.VM{}, fmt.Errorf("get domain XML description: %w", err)
	}

	cfg, err := GetConfigFromXML(xmlDesc)
	if err != nil {
		return server.VM{}, fmt.Errorf("get config from XML: %w", err)
	}

	return server.VM{
		Config: cfg,
		Status: status,
	}, nil
}

func GetConfigFromXML(xmlDesc string) (server.VMConfig, error) {
	var d Domain
	if err := xml.Unmarshal([]byte(xmlDesc), &d); err != nil {
		return server.VMConfig{}, fmt.Errorf("unmarshal XML: %w", err)
	}
	var memoryMiB uint
	switch d.Memory.Unit {
	case "KiB":
		memoryMiB = uint(d.Memory.Value) / 1024
	case "MiB":
		memoryMiB = uint(d.Memory.Value)
	case "GiB":
		memoryMiB = uint(d.Memory.Value) * 1024
	default:
		return server.VMConfig{}, fmt.Errorf("unexpected memory unit in domain XML: %s", d.Memory.Unit)
	}
	var bootISO Disk
	var primaryDisk Disk
	for _, disk := range d.Devices.Disks {
		if disk.Type == "file" && disk.Driver.Type == "raw" {
			bootISO = disk
		} else if disk.Type == "file" && disk.Driver.Type == "qcow2" {
			primaryDisk = disk
		}
	}
	if bootISO.Source.File == "" || primaryDisk.Source.File == "" {
		return server.VMConfig{}, fmt.Errorf("unexpected domain XML: missing boot ISO or primary disk")
	}
	return server.VMConfig{
		Name:      d.Name,
		VCPUs:     uint(d.VCPU.Value),
		MemoryMiB: memoryMiB,
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
