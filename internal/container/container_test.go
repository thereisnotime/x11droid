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

func TestSpawnOptsZeroValueDefaults(t *testing.T) {
	// A zero-value SpawnOpts must leave dimensions/compositor unset so the
	// container image falls back to its own defaults.
	var o SpawnOpts
	if o.Width != 0 || o.Height != 0 || o.Compositor != "" {
		t.Errorf("zero SpawnOpts should have no overrides, got %+v", o)
	}
}
