package vms

import (
	"fmt"
	"os"
	"path/filepath"

	"libvirt.org/go/libvirt"
)

func StopVM(name string, graceful bool) error {
	domain, conn, err := getDomain(name)
	if err != nil {
		return err
	}
	defer conn.Close()

	if graceful {
		return domain.Shutdown() // sends ACPI signal; guest OS shuts down cleanly
	}
	return domain.Destroy() // hard power-off
}

func RestartVM(name string, graceful bool) error {
	domain, conn, err := getDomain(name)
	if err != nil {
		return err
	}
	defer conn.Close()

	if graceful {
		return domain.Reboot(libvirt.DOMAIN_REBOOT_DEFAULT)
	}
	if err := domain.Destroy(); err != nil {
		return err
	}
	return domain.Create()
}

func UpdateVM(name string, vcpus uint, memoryMiB uint) error {
	domain, conn, err := getDomain(name)
	if err != nil {
		return err
	}
	defer conn.Close()

	// memory can be changed live but vCPUs usually require a reboot unless the guest supports hotplug
	if err := domain.SetMemory(uint64(memoryMiB) * 1024); err != nil {
		return fmt.Errorf("set memory: %w", err)
	}
	if err := domain.SetVcpus(vcpus); err != nil {
		return fmt.Errorf("set vcpus: %w", err)
	}
	return nil
}

func DeleteVM(name string) error {
	domain, conn, err := getDomain(name)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = domain.Destroy() // best-effort stop; ignore error if already off
	diskPath := filepath.Join(dataDir, "disks", name+".qcow2")
	if err := domain.UndefineFlags(libvirt.DOMAIN_UNDEFINE_NVRAM); err != nil {
		return fmt.Errorf("undefine domain: %w", err)
	}
	return os.Remove(diskPath) // kaboom kablow goes my disk
}

func GetVMStatus(name string) (string, error) {
	domain, conn, err := getDomain(name)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	state, _, err := domain.GetState()
	if err != nil {
		return "", err
	}
	switch state {
	case libvirt.DOMAIN_RUNNING:
		return "running", nil
	case libvirt.DOMAIN_PAUSED:
		return "paused", nil
	case libvirt.DOMAIN_SHUTOFF:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

// we could probably just have a global libvert conn or a struct that holds it
func getDomain(name string) (*libvirt.Domain, *libvirt.Connect, error) {
	conn, err := libvirt.NewConnect("qemu:///session")
	if err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	domain, err := conn.LookupDomainByName(name)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("domain not found: %w", err)
	}
	return domain, conn, nil
}
