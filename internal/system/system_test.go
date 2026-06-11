// SPDX-License-Identifier: GPL-3.0-only

package system

import (
	"testing"
)

func TestDetect_X11(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")

	info := Detect()
	if info.Kind != SessionX11 {
		t.Errorf("expected SessionX11, got %v", info.Kind)
	}
	if !info.IsX11() {
		t.Error("IsX11() should be true")
	}
	if info.Warning() != "" {
		t.Errorf("expected no warning, got %q", info.Warning())
	}
}

func TestDetect_Wayland(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")

	info := Detect()
	if info.Kind != SessionWayland {
		t.Errorf("expected SessionWayland, got %v", info.Kind)
	}
	if info.IsX11() {
		t.Error("IsX11() should be false")
	}
	if info.Warning() == "" {
		t.Error("expected a warning for Wayland session")
	}
}

func TestDetect_XWayland(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")

	info := Detect()
	if info.Kind != SessionXWayland {
		t.Errorf("expected SessionXWayland, got %v", info.Kind)
	}
	if info.Warning() == "" {
		t.Error("expected a warning for XWayland session")
	}
}

func TestDetect_NoDisplay(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	info := Detect()
	if info.Kind != SessionUnknown {
		t.Errorf("expected SessionUnknown, got %v", info.Kind)
	}
	if info.Warning() == "" {
		t.Error("expected a warning when DISPLAY is not set")
	}
}

func TestDetect_InferX11FromDisplay(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "")
	t.Setenv("DISPLAY", ":1")
	t.Setenv("WAYLAND_DISPLAY", "")

	info := Detect()
	if info.Kind != SessionX11 {
		t.Errorf("expected SessionX11 inferred from DISPLAY, got %v", info.Kind)
	}
}

func TestKindLabel(t *testing.T) {
	cases := []struct {
		kind  SessionKind
		label string
	}{
		{SessionX11, "X11"},
		{SessionWayland, "Wayland"},
		{SessionXWayland, "XWayland"},
		{SessionUnknown, "unknown"},
	}
	for _, c := range cases {
		info := Info{Kind: c.kind}
		if got := info.KindLabel(); got != c.label {
			t.Errorf("KindLabel(%v) = %q, want %q", c.kind, got, c.label)
		}
	}
}
