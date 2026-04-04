package vms

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	"libvirt.org/go/libvirt"
)

var goArchToLibvirtArch = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}
var goArchToMachineType = map[string]string{
	"amd64": "q35",
	"arm64": "virt",
}

func CreateVM(config *server.VMConfig) (int, error) {
	if err := validateConfig(config); err != nil {
		return 0, fmt.Errorf("invalid VM config: %w", err)
	}

	// validate that the ISO exists before doing any other work
	isoPath := filepath.Join(dataDir, "boot_files", config.BootFile)
	if _, err := os.Stat(isoPath); err != nil {
		return 0, fmt.Errorf("iso not found at %s", isoPath)
	}

	// create qcow2 disk for the VM
	diskPath := filepath.Join(dataDir, "disks", config.Name+".qcow2")
	if err := createDisk(diskPath, config.DiskGiB); err != nil {
		return 0, fmt.Errorf("create disk: %w", err)
	}

	bridge := dv(config.NetworkBridge, "virbr0") // libvirt's default NAT bridge, unused for now

	// connect to libvirt
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return 0, fmt.Errorf("connect to hypervisor: %w", err)
	}
	defer conn.Close()

	// create and define xml
	xml := buildDomainXML(config, isoPath, diskPath, bridge)
	domain, err := conn.DomainDefineXML(xml)
	if err != nil {
		return 0, fmt.Errorf("define domain: %w", err)
	}

	if err := domain.Create(); err != nil { // create and start the VM
		domain.Undefine()
		os.Remove(diskPath)
		return 0, fmt.Errorf("start domain: %w", err)
	}

	xmlDesc, err := domain.GetXMLDesc(0)
	if err != nil {
		return 0, fmt.Errorf("get domain XML description: %w", err)
	}
	vncPort, err := vncPortFromXML(xmlDesc)
	if err != nil {
		return 0, fmt.Errorf("parse VNC port from domain XML: %w", err)
	}

	return vncPort, nil
}

// note: bridge is unused
func buildDomainXML(c *server.VMConfig, isoPath, diskPath, bridge string) string {
	var firmwareXML string
	var extraFeatures string
	if runtime.GOARCH == "arm64" {
		slog.Info("enabling UEFI firmware and GICv3 for arm64 VM")
		// for aarch64, we need to specify UEFI firmware
		firmwareXML = `<loader readonly='yes' type='pflash'>/usr/share/AAVMF/AAVMF_CODE.fd</loader>
  	<nvram template='/usr/share/AAVMF/AAVMF_VARS.fd'>/var/lib/libvirt/qemu/nvram/yogurt_VARS.fd</nvram>`
		// and we need to enable some extra features
		extraFeatures = `<gic version='2'/>`
	} else {
		extraFeatures = "<apic/>" // enables amd apic which is cool if on x86_64
	}
	x := fmt.Sprintf(`
<domain type='kvm'>
  <name>%s</name>
  <memory unit='MiB'>%d</memory>
  <currentMemory unit='MiB'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>

  <os>
    <type arch='%s' machine='%s'>hvm</type>
	%s
    <bootmenu enable='yes'/>
  </os>

  <features>
    <acpi/>  <!-- lets the guest OS respond to shutdown signals -->
	%s
  </features>

  <cpu mode='host-passthrough'/>  <!-- best performance; guest sees real CPU -->

  <clock offset='utc'>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='hpet' present='no'/>
  </clock>

  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>

  <devices>
	<controller type='usb' index='0' model='qemu-xhci'>
    </controller>
	<input type='keyboard' bus='usb'/>
	<input type='mouse' bus='usb'/>

    <!-- Primary disk (persistent, writable) -->
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2' cache='writeback'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
	  <boot order='2'/>
    </disk>

    <!-- Boot ISO -->
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='vdb' bus='virtio'/>
      <readonly/>
	  <boot order='1'/>
    </disk>

    <!-- Networking via NAT bridge -->
    <interface type='network'>
      <source network='default'/>
      <model type='virtio'/>
    </interface>

    <graphics type='vnc' port='-1' autoport='yes' listen='0.0.0.0' passwd='password'>
      <listen type='address' address='0.0.0.0'/>
    </graphics>
    <video>
      <model type='virtio' vram='16384'/>  <!-- vga is broadly compatible -->
    </video>

    <!-- tablet input fixes mouse cursor alignment in VNC -->
    <input type='tablet' bus='usb'/>

    <!-- guest agent channel for graceful shutdown -->
    <channel type='unix'>
      <target type='virtio' name='org.qemu.guest_agent.0'/>
    </channel>

    <memballoon model='virtio'/>  <!-- allows runtime memory adjustment -->
  </devices>
</domain>`,
		c.Name,
		c.MemoryMiB,
		c.MemoryMiB,
		c.VCPUs,
		goArchToLibvirtArch[runtime.GOARCH],
		goArchToMachineType[runtime.GOARCH],
		firmwareXML,
		extraFeatures,
		diskPath,
		isoPath,
	)
	os.WriteFile(filepath.Join(dataDir, c.Name+".xml"), []byte(x), 0644) // for debugging
	return x
}

func createDisk(path string, sizeGiB uint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// qemu-img must be installed on the host (it's part of qemu-utils i think)
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, fmt.Sprintf("%dG", sizeGiB))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img create disk: %s: %w", out, err)
	}
	return nil
}

func validateConfig(config *server.VMConfig) error {
	if len(config.Name) > 32 || !alphanumericRegexp.MatchString(config.Name) {
		return fmt.Errorf("invalid VM name (should be alphanumeric and less than 32 characters)")
	}
	if config.VCPUs < 1 || config.VCPUs > MAX_VCPU {
		return fmt.Errorf("invalid VM vcpus (should be between 1 and %d, inclusive)", MAX_VCPU)
	}
	if config.MemoryMiB < 128 || config.MemoryMiB > MAX_MEMORY_MiB {
		return fmt.Errorf("invalid VM memory (should be between 128 MiB and %d MiB)", MAX_MEMORY_MiB)
	}
	// NOTE: right now boot files must be ISOs
	// if after, found := strings.CutSuffix(config.BootFile, ".iso"); !found || len(after) == 0 || len(after) > 64 || !alphanumericRegexp.MatchString(after) {
	// 	return fmt.Errorf("invalid VM boot file name (should be alphanumeric and less than 64 characters)")
	// }
	if config.DiskGiB < 1 || config.DiskGiB > MAX_DISK_GiB {
		return fmt.Errorf("invalid VM disk size (should be between 1 GiB and %d GiB)", MAX_DISK_GiB)
	}
	return nil
}
