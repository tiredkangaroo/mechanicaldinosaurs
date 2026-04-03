package vms

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"runtime"
)

var MAX_MEMORY_MiB = uint(32768) // NOTE: this should be adjusted to something reasonable for the host machine
var MAX_DISK_GiB = uint(500)     // NOTE: see above
var MAX_VCPU = uint(runtime.NumCPU())
var dataDir = os.Getenv("MECHANICAL_DINOSAURS_DATA")
var alphanumericRegexp = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

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
