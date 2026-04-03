package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

var alphanumericRegexp = regexp.MustCompile(`^[a-zA-Z0-9]*$`)

func ListServices() ([]server.Service, error) {
	output, err := exec.Command("systemctl", "list-units", "--all", "--type=service", "--state=running,failed,exited,dead").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("malformed systemctl data")
	}
	services := make([]server.Service, 0, len(lines))
	for _, line := range lines[1:] { // skips over column names
		fields := strings.Fields(line)
		if len(fields) < 5 {
			break // end of services list
		}
		skipIndexes := 0
		if !strings.HasSuffix(fields[0], ".service") { // sometimes the first part of the line is not service name (e.g: ● on failed units)
			if len(fields) < 6 {
				continue // cannot parse this line
			}
			skipIndexes = 1
		}
		if fields[skipIndexes+1] != "loaded" {
			continue // not-found or whatever else may be in this field, ignore
		}
		services = append(services, server.Service{
			Name:        fields[skipIndexes],
			Status:      fields[skipIndexes+3],
			Description: strings.Join(fields[skipIndexes+4:], " "),
		})
	}
	return services[0:], nil
}

func GetServiceContent(name string) (string, error) {
	if !validateServiceName(name) {
		return "", fmt.Errorf("invalid service name")
	}
	// this is an admin endpoint anyway, but we should still sanitize
	data, err := os.ReadFile(filepath.Join("/etc/systemd/system/", name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CreateService(name, content string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	err := os.WriteFile(filepath.Join("/etc/systemd/system/", name), []byte(content), 0744)
	if err != nil {
		return err
	}
	return exec.Command("systemctl", "daemon-reload").Run()
}

func DeleteService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	err := os.Remove(filepath.Join("/etc/systemd/system/", name))
	if err != nil {
		return err
	}
	return exec.Command("systemctl", "daemon-reload").Run()
}

func EnableService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	return exec.Command("systemctl", "enable", name).Run()
}

func DisableService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	return exec.Command("systemctl", "disable", name).Run()
}

func StartService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	return exec.Command("systemctl", "start", name).Run()
}

func StopService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	return exec.Command("systemctl", "stop", name).Run()
}

func RestartService(name string) error {
	if !validateServiceName(name) {
		return fmt.Errorf("invalid service name")
	}
	return exec.Command("systemctl", "restart", name).Run()
}

func GetServiceLogs(name string) (io.ReadCloser, error) {
	if !validateServiceName(name) {
		return nil, fmt.Errorf("invalid service name")
	}
	cmd := exec.Command("journalctl", "-u", name)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return pipe, nil
}

func validateServiceName(name string) bool {
	after, found := strings.CutSuffix(name, ".service")
	return found && alphanumericRegexp.MatchString(after)
}
