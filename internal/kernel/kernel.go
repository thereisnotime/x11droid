package kernel

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// binder_linux is required. ashmem_linux is only needed on kernels < 5.18;
// newer kernels have it built-in (CONFIG_ASHMEM=y) so modprobe will fail
// with "not found" and that is fine.
var required = []string{"binder_linux"}
var optional = []string{"ashmem_linux"}

type ModuleState int

const (
	StateLoaded   ModuleState = iota // loaded as a kernel module
	StateMissing                     // not loaded, not built-in
	StateBuiltIn                     // not a module but available via built-in
	StateOptional                    // not present and not needed (kernel >= 5.18)
)

type ModuleStatus struct {
	Name     string
	State    ModuleState
	Required bool
}

func (s ModuleStatus) OK() bool {
	return s.State == StateLoaded || s.State == StateBuiltIn || s.State == StateOptional
}

func Status() []ModuleStatus {
	loaded := loadedModules()
	var out []ModuleStatus

	for _, m := range required {
		state := StateMissing
		if loaded[m] {
			state = StateLoaded
		}
		out = append(out, ModuleStatus{Name: m, State: state, Required: true})
	}

	for _, m := range optional {
		state := StateOptional
		if loaded[m] {
			state = StateLoaded
		}
		out = append(out, ModuleStatus{Name: m, State: state, Required: false})
	}

	return out
}

func AllLoaded() bool {
	for _, s := range Status() {
		if s.Required && !s.OK() {
			return false
		}
	}
	return true
}

func Load() error {
	mods := append(required, optional...)
	for _, m := range mods {
		out, err := exec.Command("sudo", "modprobe", m).CombinedOutput()
		if err != nil {
			if m != required[0] && strings.Contains(string(out), "not found") {
				continue
			}
			return fmt.Errorf("modprobe %s: %w\n%s", m, err, out)
		}
	}
	return nil
}

func Unload() error {
	mods := append(required, optional...)
	for i := len(mods) - 1; i >= 0; i-- {
		m := mods[i]
		out, err := exec.Command("sudo", "rmmod", m).CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "not currently loaded") ||
				strings.Contains(string(out), "not found") {
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

// loadedModules returns the set of currently loaded module names.
func loadedModules() map[string]bool {
	data, err := os.ReadFile("/proc/modules")
	out := map[string]bool{}
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if parts := strings.Fields(line); len(parts) > 0 {
			out[parts[0]] = true
		}
	}
	return out
}

