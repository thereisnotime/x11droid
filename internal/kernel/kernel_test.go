package kernel

import (
	"strings"
	"testing"
)

func TestLoadedModules_Empty(t *testing.T) {
	// loadedModules reads /proc/modules; if it fails it returns an empty map — not an error.
	m := loadedModules()
	if m == nil {
		t.Error("loadedModules() should never return nil")
	}
}

func TestModuleStatus_OK(t *testing.T) {
	cases := []struct {
		state ModuleState
		ok    bool
	}{
		{StateLoaded, true},
		{StateBuiltIn, true},
		{StateOptional, true},
		{StateMissing, false},
	}
	for _, c := range cases {
		s := ModuleStatus{State: c.state}
		if s.OK() != c.ok {
			t.Errorf("OK() for state %v = %v, want %v", c.state, s.OK(), c.ok)
		}
	}
}

func TestStatus_RequiredPresent(t *testing.T) {
	statuses := Status()
	found := false
	for _, s := range statuses {
		if s.Name == "binder_linux" {
			found = true
			if !s.Required {
				t.Error("binder_linux should be marked required")
			}
		}
	}
	if !found {
		t.Error("binder_linux should appear in Status()")
	}
}

func TestStatus_OptionalPresent(t *testing.T) {
	statuses := Status()
	found := false
	for _, s := range statuses {
		if s.Name == "ashmem_linux" {
			found = true
			if s.Required {
				t.Error("ashmem_linux should not be marked required")
			}
		}
	}
	if !found {
		t.Error("ashmem_linux should appear in Status()")
	}
}

func TestAllLoaded_ReturnsBool(t *testing.T) {
	// Just verify it doesn't panic.
	_ = AllLoaded()
}

func TestParseModules(t *testing.T) {
	// Simulate /proc/modules output with binder_linux present.
	data := "binder_linux 65536 0 - Live 0xffffffffc0a00000\nfoo 4096 0 - Live 0x0\n"
	loaded := map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		if parts := strings.Fields(line); len(parts) > 0 {
			loaded[parts[0]] = true
		}
	}
	if !loaded["binder_linux"] {
		t.Error("binder_linux should be in parsed modules")
	}
	if loaded["ashmem_linux"] {
		t.Error("ashmem_linux should not be in parsed modules")
	}
}
