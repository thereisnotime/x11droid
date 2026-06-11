// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.Width <= 0 || d.Height <= 0 {
		t.Errorf("default dimensions must be positive, got %dx%d", d.Width, d.Height)
	}
	if d.Orientation != Portrait {
		t.Errorf("default orientation = %q, want %q", d.Orientation, Portrait)
	}
	if d.Compositor != CompositorAuto {
		t.Errorf("default compositor = %q, want %q", d.Compositor, CompositorAuto)
	}
}

func TestEffectiveDims(t *testing.T) {
	cases := []struct {
		name   string
		w, h   int
		orient string
		wantW  int
		wantH  int
	}{
		{"portrait keeps tall", 540, 960, Portrait, 540, 960},
		{"portrait normalizes swapped input", 960, 540, Portrait, 540, 960},
		{"landscape swaps to wide", 540, 960, Landscape, 960, 540},
		{"landscape from wide input", 960, 540, Landscape, 960, 540},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := Config{Width: c.w, Height: c.h, Orientation: c.orient}
			gotW, gotH := cfg.EffectiveDims()
			if gotW != c.wantW || gotH != c.wantH {
				t.Errorf("EffectiveDims() = %dx%d, want %dx%d", gotW, gotH, c.wantW, c.wantH)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	d := Default()
	cfg := Config{Width: 0, Height: -5, Orientation: "sideways", Compositor: "bogus"}
	cfg.normalize()
	if cfg.Width != d.Width || cfg.Height != d.Height {
		t.Errorf("normalize did not restore dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Orientation != d.Orientation {
		t.Errorf("normalize orientation = %q, want %q", cfg.Orientation, d.Orientation)
	}
	if cfg.Compositor != d.Compositor {
		t.Errorf("normalize compositor = %q, want %q", cfg.Compositor, d.Compositor)
	}
}

func TestNormalizeKeepsValidValues(t *testing.T) {
	cfg := Config{Width: 720, Height: 1280, Orientation: Landscape, Compositor: CompositorWeston}
	cfg.normalize()
	if cfg.Width != 720 || cfg.Height != 1280 || cfg.Orientation != Landscape || cfg.Compositor != CompositorWeston {
		t.Errorf("normalize clobbered valid config: %+v", cfg)
	}
}

// TestSaveLoadRoundTrip verifies persistence by pointing HOME at a temp dir so
// the config lands somewhere isolated and reads back identically.
func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	want := Config{Width: 720, Height: 1280, Orientation: Landscape, Compositor: CompositorCage}
	if err := want.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Sanity: the file should exist under the temp HOME.
	if _, err := os.Stat(filepath.Join(tmp, ".config", "x11droid", "config.json")); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	got := Load()
	if got != want {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := Load(); got != Default() {
		t.Errorf("Load() with no file = %+v, want default %+v", got, Default())
	}
}
