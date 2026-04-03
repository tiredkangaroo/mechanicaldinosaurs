package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

func GetServerInfo() (*server.Info, error) {
	info := &server.Info{}

	// host info stuff
	hostInfo, err := host.Info()
	if err == nil { // if we can't get host info, we can still return other info (although something is probably very wrong)
		info.Hostname = hostInfo.Hostname
		info.OS = hostInfo.OS
		info.OSRelease = hostInfo.PlatformVersion
		info.Uptime = hostInfo.Uptime
	} else {
		slog.Warn("get host info failed", "error", err)
	}

	// cpu info stuff
	info.CPU, _ = fieldValueFromProcFile("/proc/cpuinfo", "model name")
	info.CPUArch = runtime.GOARCH
	info.CPUNum = runtime.NumCPU()
	if cpuUsage, err := cpu.Percent(time.Millisecond*25, false); err == nil {
		info.CPUUsage = cpuUsage[0]
	}
	info.CPUTemp, _ = getCPUTemp()

	// memory info stuff
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.MemoryCapacity = memInfo.Total
		info.MemoryUsed = memInfo.Used
	}

	// storage info stuff
	if diskUsage, err := disk.Usage("/"); err == nil {
		info.StorageCapacity = diskUsage.Total
		info.StorageUsed = diskUsage.Used
	}

	// battery info stuff
	info.HasBattery, info.Battery = getBatteryInfo()

	return info, nil
}

func getCPUTemp() (float64, error) {
	// this should be in celsius, if it's not, then idk lol
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return -1, err
	}
	temp, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1, err
	}
	return float64(temp) / 1000.0, nil
}

func getBatteryInfo() (bool, string) {
	base := "/sys/class/power_supply/"
	entries, err := os.ReadDir(base)
	if err != nil {
		return false, ""
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "BAT") {
			battFile := base + entry.Name() + "/model_name"
			data, err := os.ReadFile(battFile)
			if err != nil {
				return true, ""
			}
			return true, strings.TrimSpace(string(data))
		}
	}
	return false, ""
}

func fieldValueFromProcFile(filename string, field string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "unknown", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, field) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "unknown", fmt.Errorf("field %s not found in %s", field, filename)
}
