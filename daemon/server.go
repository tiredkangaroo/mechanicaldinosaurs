package main

import (
	"fmt"

	"github.com/tiredkangaroo/mechanicaldinosaurs/daemon/vms"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

func main() {
	virtualMachines, err := vms.ListVMs()
	if err != nil {
		fmt.Println("error listing VMs:", err)
		return
	}
	for _, vm := range virtualMachines {
		fmt.Printf(
			"VM: %s, Status: %s, VCPUs: %d, MemoryMiB: %d, BootFile: %s\n",
			vm.Config.Name, vm.Status, vm.Config.VCPUs, vm.Config.MemoryMiB, vm.Config.BootFile,
		)
		fmt.Println("-- stopping VM --")
		err = vms.StopVM(vm.Config.Name, true) // try graceful shutdown
		if err != nil {
			fmt.Println("error stopping VM:", err)
		}
		fmt.Println("-- deleting VM --")
		err = vms.DeleteVM(vm.Config.Name)
		if err != nil {
			fmt.Println("error deleting VM:", err)
		}
	}
	fmt.Println("-- creating VM --")
	port, err := vms.CreateVM(&server.VMConfig{
		Name:      "yogurt",
		VCPUs:     8,
		MemoryMiB: 2048,
		DiskGiB:   1,
		BootFile:  "ubuntu-25.10-desktop-arm64.iso",
	})
	if err != nil {
		fmt.Println("error creating VM:", err)
	}
	fmt.Println("Created VM on port:", port)
}
