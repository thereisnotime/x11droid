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
