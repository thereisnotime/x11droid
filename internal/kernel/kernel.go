// SPDX-License-Identifier: GPL-3.0-only

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

// binderDevices are the device nodes waydroid expects. Kernels built with an
// empty CONFIG_ANDROID_BINDER_DEVICES create none of them unless binder_linux
// is loaded with devices=binder,hwbinder,vndbinder.
var binderDevices = []string{"/dev/binder", "/dev/hwbinder", "/dev/vndbinder"}

// BinderDeviceNodes returns the device nodes waydroid needs.
func BinderDeviceNodes() []string { return binderDevices }

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

// Load ensures binder_linux is loaded so binderfs is available. The container
// provisions its own binder nodes via binderfs (this kernel ships
// CONFIG_ANDROID_BINDER_DEVICES="" and creates none statically), so no device
// parameter or chmod is needed on the host. Runs as root — x11droid runs under
// sudo — so no sudo prefix.
func Load() error {
	if !loadedModules()["binder_linux"] {
		out, err := exec.Command("modprobe", "binder_linux").CombinedOutput()
		if err != nil {
			return fmt.Errorf("modprobe binder_linux: %w\n%s", err, out)
		}
	}
	// ashmem_linux is optional — absent (built-in) on kernels >= 5.18.
	_ = exec.Command("modprobe", "ashmem_linux").Run()
	return nil
}

func Unload() error {
	mods := append(required, optional...)
	for i := len(mods) - 1; i >= 0; i-- {
		m := mods[i]
		out, err := exec.Command("rmmod", m).CombinedOutput()
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
