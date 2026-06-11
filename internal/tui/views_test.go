// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"
	"testing"
)

func TestRamMB(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"megabytes", "18.77MB", 18.77},
		{"gigabytes scales by 1024", "1.23GB", 1.23 * 1024},
		{"gibibytes scales by 1024", "1GiB", 1024},
		{"kilobytes scales down", "512kB", 512.0 / 1024},
		{"kibibytes scales down", "512KiB", 512.0 / 1024},
		{"plain dash is zero", "-", 0},
		{"empty is zero", "", 0},
		{"plain number no unit", "42", 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ramMB(c.in)
			if got != c.want {
				t.Errorf("ramMB(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestAndroidStopped(t *testing.T) {
	cases := []struct {
		name   string
		status string
		ram    string
		want   bool
	}{
		{"up with tiny ram is stopped", "Up 5 minutes", "18.77MB", true},
		{"up with large ram is not stopped", "Up", "839MB", false},
		{"exited with dash ram is not stopped", "Exited (0)", "-", false},
		{"up with dash ram (0) is not stopped", "Up", "-", false},
		{"up at lower bound 0 excluded", "Up", "0MB", false},
		{"up just under 80 is stopped", "Up", "79MB", true},
		{"up at 80 excluded", "Up", "80MB", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := androidStopped(c.status, c.ram)
			if got != c.want {
				t.Errorf("androidStopped(%q, %q) = %v, want %v", c.status, c.ram, got, c.want)
			}
		})
	}
}

func TestConfirmQuestion(t *testing.T) {
	cases := []struct {
		name     string
		action   string
		instName string
		contains []string
	}{
		{"purge mentions all its data", "Purge", "foo", []string{"Purge", "all its data", "foo"}},
		{"prune mentions orphan", "Prune Orphan Data", "ignored", []string{"orphan"}},
		{"default remove keeps data", "Remove", "bar", []string{"Remove", "data kept", "bar"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := confirmQuestion(c.action, c.instName)
			for _, sub := range c.contains {
				if !strings.Contains(got, sub) {
					t.Errorf("confirmQuestion(%q, %q) = %q, missing %q", c.action, c.instName, got, sub)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"shorter than n unchanged", "abc", 5, "abc"},
		{"exactly n unchanged", "abcde", 5, "abcde"},
		{"longer than n gets ellipsis", "abcdef", 5, "abcd…"},
		{"much longer", "hello world", 5, "hell…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.in, c.n)
			if got != c.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"shorter than n padded", "ab", 5, "ab   "},
		{"exactly n unchanged", "abcde", 5, "abcde"},
		{"longer than n unchanged", "abcdef", 5, "abcdef"},
		{"empty padded full width", "", 3, "   "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := padRight(c.in, c.n)
			if got != c.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}

func TestSizeSuffix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"non-empty gets comma prefix", "1.2 GB", ", 1.2 GB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sizeSuffix(c.in)
			if got != c.want {
				t.Errorf("sizeSuffix(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
