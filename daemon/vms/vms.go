package vms

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
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
		cfg, diskUsed, err := GetConfigFromXML(xmlDesc)
		if err != nil {
			slog.Error("get config from XML", "name", name, "error", err)
			continue
		}
		vms = append(vms, server.VM{
			Config:      cfg,
			Status:      status,
			DiskUsedGiB: uint(diskUsed / (1024 * 1024 * 1024)),
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

	err = domain.SetMemoryStatsPeriod(5, libvirt.DOMAIN_MEM_LIVE)
	if err != nil {
		return server.VM{}, fmt.Errorf("failed to set memory stats period: %w", err)
	}

	var unusedMemKiB, availableMemKiB uint64
	if status == "running" {
		fmt.Println("vm running; getting memory stats")
		stats, err := domain.MemoryStats(20, 0)
		if err != nil {
			return server.VM{}, fmt.Errorf("failed to get memory stats: %w", err)
		}
		fmt.Println("got", len(stats), "memory stats")
		for _, stat := range stats {
			fmt.Println("stat.Tag:", stat.Tag, "stat.Val:", stat.Val)
			switch libvirt.DomainMemoryStatTags(stat.Tag) {
			case libvirt.DOMAIN_MEMORY_STAT_UNUSED:
				unusedMemKiB = stat.Val
			case libvirt.DOMAIN_MEMORY_STAT_AVAILABLE:
				availableMemKiB = stat.Val
			}
		}
	} else {
		fmt.Println("vm not running; skipping memory stats")
	}
	fmt.Println("unusedMemKiB:", unusedMemKiB)
	fmt.Println("availableMemKiB:", availableMemKiB)

	xmlDesc, err := domain.GetXMLDesc(0)
	if err != nil {
		return server.VM{}, fmt.Errorf("get domain XML description: %w", err)
	}

	cfg, diskUsedKiB, err := GetConfigFromXML(xmlDesc)
	if err != nil {
		return server.VM{}, fmt.Errorf("get config from XML: %w", err)
	}
	fmt.Println("vm", name, "diskUsedKiB:", diskUsedKiB)

	// note: this function needs to populate the size of the disk by getting the primary disk file (boot order 2)
	// from the config and getting the size of it. idk how to do that but it might be on qemu-img

	return server.VM{
		Config:        cfg,
		Status:        status,
		DiskUsedGiB:   uint(diskUsedKiB / (1024 * 1024 * 1024)),
		MemoryUsedMiB: cfg.MemoryMiB - uint(unusedMemKiB/1024),
	}, nil
}

// returns actualSize & virtualSize of disk in that order (in KiB)
func GetDiskSize(diskPath string) (int64, int64, error) {
	cmd := exec.Command("qemu-img", "info", "--output=json", "--force-share", diskPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("qemu-img info failed: %s: %w", out, err)
	}

	var info struct {
		VirtualSize int64  `json:"virtual-size"`
		ActualSize  int64  `json:"actual-size"`
		Format      string `json:"format"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return 0, 0, fmt.Errorf("failed to parse qemu-img JSON: %w", err)
	}

	// numbers are in kib
	return info.ActualSize, info.VirtualSize, nil
}

func GetConfigFromXML(xmlDesc string) (server.VMConfig, int64, error) {
	var d Domain
	if err := xml.Unmarshal([]byte(xmlDesc), &d); err != nil {
		return server.VMConfig{}, 0, fmt.Errorf("unmarshal XML: %w", err)
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
		return server.VMConfig{}, 0, fmt.Errorf("unexpected memory unit in domain XML: %s", d.Memory.Unit)
	}
	var bootISO Disk
	var primaryDisk Disk
	for _, disk := range d.Devices.Disks {
		if disk.Type == "file" && disk.Driver.Type == "raw" && disk.Boot.Order == 2 {
			bootISO = disk
		} else if disk.Type == "file" && disk.Driver.Type == "qcow2" {
			primaryDisk = disk
		}
	}
	if bootISO.Source.File == "" || primaryDisk.Source.File == "" {
		return server.VMConfig{}, 0, fmt.Errorf("unexpected domain XML: missing boot ISO or primary disk")
	}
	var graphicsType string
	for _, g := range d.Devices.Graphics {
		if g.Type == "vnc" || g.Type == "spice" {
			graphicsType = g.Type
			break
		}
	}
	actualSize, virtualSize, err := GetDiskSize(primaryDisk.Source.File)
	if err != nil {
		return server.VMConfig{}, 0, fmt.Errorf("get disk size: %w", err)
	}
	return server.VMConfig{
		Name:         d.Name,
		VCPUs:        uint(d.VCPU.Value),
		MemoryMiB:    memoryMiB,
		BootFile:     filepath.Base(bootISO.Source.File),
		GraphicsType: graphicsType,
		DiskGiB:      uint(virtualSize / (1024 * 1024 * 1024)), // convert bytes to GiB
	}, actualSize, nil
}

// util functio nstuff

func dv(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// this works for spice as well
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
		if g.Port > 0 {
			return g.Port, nil
		}
	}
	return 0, fmt.Errorf("no vnc graphics found")
}

// proxy vm, blocks until conn is closed
func ProxyVM(name string, conn io.ReadWriteCloser) error {
	// get the port from domain
	connLibvirt, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("connect to hypervisor: %w", err)
	}
	defer connLibvirt.Close()

	domain, err := connLibvirt.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("lookup domain: %w", err)
	}

	xmlDesc, err := domain.GetXMLDesc(0)
	if err != nil {
		return fmt.Errorf("get domain XML description: %w", err)
	}

	port, err := vncPortFromXML(xmlDesc)
	if err != nil {
		return fmt.Errorf("get VNC port from XML: %w", err)
	}

	// connect to the server
	connVNC, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("connect to VNC server: %w", err)
	}
	defer connVNC.Close()

	// proxy the connection
	go func() {
		defer conn.Close()
		defer connVNC.Close()
		io.Copy(conn, connVNC)
	}()
	io.Copy(connVNC, conn)

	return nil
}
