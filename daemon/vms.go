package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	"libvirt.org/go/libvirt"
)

var MAX_MEMORY_MiB = uint(32768) // NOTE: this should be adjusted to something reasonable for the host machine
var MAX_VCPU = uint(runtime.NumCPU())

func CreateVM(config *server.VMConfig) error {
	if len(config.Name) > 32 || !alphanumericRegexp.MatchString(config.Name) {
		return fmt.Errorf("invalid VM name (should be alphanumeric and less than 32 characters)")
	}
	if config.VCPUs < 1 || config.VCPUs > MAX_VCPU {
		return fmt.Errorf("invalid VM vcpus (should be between 1 and %d, inclusive)", MAX_VCPU)
	}
	if config.MemoryMiB < 128 || config.MemoryMiB > MAX_MEMORY_MiB {
		return fmt.Errorf("invalid VM memory (should be between 128 MiB and %d MiB)", MAX_MEMORY_MiB)
	}
	if after, found := strings.CutSuffix(config.ISOName, ".iso"); !found || len(after) == 0 || len(after) > 64 || !alphanumericRegexp.MatchString(after) {
		return fmt.Errorf("invalid VM ISO name (should be alphanumeric and less than 64 characters)")
	}

	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("connect to hypervisor: %w", err)
	}
	defer conn.Close()

	xml := fmt.Sprintf(`
	<domain type='kvm'>
		<name>%s</name>
		<memory unit='MiB'>%d</memory>
		<vcpu placement='static'>%d</vcpu>
		<os>
			<type arch='%s'>hvm</type>
			<boot dev='cdrom'/>
		</os>
		<devices>
			<disk type='file' device='cdrom'>
	      		<source file='/path/to/your.iso'/>
	      		<target dev='hdc' bus='ide'/>
	    	</disk>
			<graphics type='vnc' port='6700' autoport='yes' listen='127.0.0.1'/>
		</devices>
	</domain>
	`)
	domain, err := conn.DomainDefineXML(xml)
	if err != nil {
		return fmt.Errorf("define domain: %w", err)
	}
	err = domain.CreateWithFlags(libvirt.DOMAIN_START_PAUSED)
	if err != nil {
		return fmt.Errorf("create domain: %w", err)
	}
	return nil
}
