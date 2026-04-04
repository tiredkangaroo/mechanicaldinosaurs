package main

import (
	"fmt"

	"github.com/tiredkangaroo/mechanicaldinosaurs/daemon/vms"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

func main() {
	fmt.Println(vms.ListVMs())
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
