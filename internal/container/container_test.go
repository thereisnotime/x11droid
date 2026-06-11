// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"testing"
)

func TestIsSudoAuthErr(t *testing.T) {
	cases := []struct {
		out  string
		want bool
	}{
		{"sudo: a password is required\n", true},
		{"sudo: no password supplied\n", true},
		{"sudo: this command requires a password\n", true},
		{"Error: no such container\n", false},
		{"", false},
		{"permission denied", false},
	}
	for _, c := range cases {
		got := isSudoAuthErr([]byte(c.out))
		if got != c.want {
			t.Errorf("isSudoAuthErr(%q) = %v, want %v", c.out, got, c.want)
		}
	}
}

func TestInstanceDataDir_NotEmpty(t *testing.T) {
	dir := instanceDataDir("test-instance")
	if dir == "" {
		t.Error("instanceDataDir should not return empty string")
	}
	if dir == "test-instance" {
		t.Error("instanceDataDir should return an absolute path")
	}
}

func TestPodmanInstalled(t *testing.T) {
	// Just verify the function doesn't panic.
	_ = PodmanInstalled()
}

func TestInstanceIsRunning(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"Up 2 hours", true},
		{"Up About a minute", true},
		{"Exited (0) 3 minutes ago", false},
		{"Exited (1) 5 seconds ago", false},
		{"Created", false},
		{"", false},
	}
	for _, c := range cases {
		got := Instance{Status: c.status}.IsRunning()
		if got != c.want {
			t.Errorf("Instance{Status:%q}.IsRunning() = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 MB"},
		{"512 MB", 512 << 20, "512 MB"},
		{"just under 1 GB stays MB", (1 << 30) - 1, "1023 MB"},
		{"exactly 1 GB", 1 << 30, "1.0 GB"},
		{"1.5 GB", (1 << 30) + (1 << 29), "1.5 GB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := humanBytes(c.in)
			if got != c.want {
				t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSelectedApps(t *testing.T) {
	cases := []struct {
		name string
		opts SpawnOpts
		want string
	}{
		{"none selected", SpawnOpts{}, ""},
		{"only fdroid", SpawnOpts{FDroid: true}, "fdroid"},
		{"only shelter", SpawnOpts{Shelter: true}, "shelter"},
		{"fdroid and obtainium keep order", SpawnOpts{FDroid: true, Obtainium: true}, "fdroid,obtainium"},
		{"all four in order", SpawnOpts{FDroid: true, Aurora: true, Obtainium: true, Shelter: true}, "fdroid,aurora,obtainium,shelter"},
		{"aurora and shelter", SpawnOpts{Aurora: true, Shelter: true}, "aurora,shelter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := selectedApps(c.opts)
			if got != c.want {
				t.Errorf("selectedApps(%+v) = %q, want %q", c.opts, got, c.want)
			}
		})
	}
}

func TestSpawnOptsZeroValueDefaults(t *testing.T) {
	// A zero-value SpawnOpts must leave dimensions/compositor unset so the
	// container image falls back to its own defaults.
	var o SpawnOpts
	if o.Width != 0 || o.Height != 0 || o.Compositor != "" {
		t.Errorf("zero SpawnOpts should have no overrides, got %+v", o)
	}
}
