package kernel

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var requiredModules = []string{"binder_linux", "ashmem_linux"}

type ModuleStatus struct {
	Name   string
	Loaded bool
}

func Status() []ModuleStatus {
	data, err := os.ReadFile("/proc/modules")
	loaded := map[string]bool{}
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if parts := strings.Fields(line); len(parts) > 0 {
				loaded[parts[0]] = true
			}
		}
	}
	out := make([]ModuleStatus, len(requiredModules))
	for i, m := range requiredModules {
		out[i] = ModuleStatus{Name: m, Loaded: loaded[m]}
	}
	return out
}

func AllLoaded() bool {
	for _, s := range Status() {
		if !s.Loaded {
			return false
		}
	}
	return true
}

func Load() error {
	for _, m := range requiredModules {
		if out, err := exec.Command("sudo", "modprobe", m).CombinedOutput(); err != nil {
			// ashmem may not exist on newer kernels — skip if not found
			if m == "ashmem_linux" && strings.Contains(string(out), "not found") {
				continue
			}
			return fmt.Errorf("modprobe %s: %w\n%s", m, err, out)
		}
	}
	return nil
}

func Unload() error {
	for i := len(requiredModules) - 1; i >= 0; i-- {
		m := requiredModules[i]
		if out, err := exec.Command("sudo", "rmmod", m).CombinedOutput(); err != nil {
			if strings.Contains(string(out), "not currently loaded") {
				continue
			}
			return fmt.Errorf("rmmod %s: %w\n%s", m, err, out)
		}
	}
	return nil
}

func BinderDeviceExists() bool {
	_, err := os.Stat("/dev/binder")
	return err == nil
}
