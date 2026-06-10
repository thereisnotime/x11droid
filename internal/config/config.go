// Package config persists user-tunable defaults for new instances —
// window resolution, orientation and the compositor to use — to
// ~/.config/x11droid/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Orientation values.
const (
	Portrait  = "portrait"
	Landscape = "landscape"
)

// Compositor values. "auto" probes cage and falls back to weston (cage's
// wlroots X11 backend fails on NVIDIA, weston's pixman backend does not).
const (
	CompositorAuto   = "auto"
	CompositorWeston = "weston"
	CompositorCage   = "cage"
)

// Config holds the persisted defaults applied to every new instance.
type Config struct {
	// Width and Height are stored as the portrait (tall) dimensions;
	// EffectiveDims swaps them when Orientation is landscape.
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Orientation string `json:"orientation"`
	Compositor  string `json:"compositor"`
}

// Default returns the built-in defaults used before anything is saved.
func Default() Config {
	return Config{
		Width:       540,
		Height:      960,
		Orientation: Portrait,
		Compositor:  CompositorAuto,
	}
}

func path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", "x11droid", "config.json")
	}
	return filepath.Join(home, ".config", "x11droid", "config.json")
}

// Load reads the saved config, falling back to defaults for any missing or
// invalid field. A missing file is not an error — defaults are returned.
func Load() Config {
	cfg := Default()
	data, err := os.ReadFile(path())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	cfg.normalize()
	return cfg
}

// Save writes the config to disk, creating the config directory if needed.
func (c Config) Save() error {
	c.normalize()
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// EffectiveDims returns the width/height to hand the compositor, swapped for
// landscape so the longer edge becomes the width.
func (c Config) EffectiveDims() (w, h int) {
	lo, hi := c.Width, c.Height
	if lo > hi {
		lo, hi = hi, lo
	}
	if c.Orientation == Landscape {
		return hi, lo
	}
	return lo, hi
}

// normalize clamps unset or nonsensical values back to safe defaults.
func (c *Config) normalize() {
	d := Default()
	if c.Width <= 0 {
		c.Width = d.Width
	}
	if c.Height <= 0 {
		c.Height = d.Height
	}
	if c.Orientation != Portrait && c.Orientation != Landscape {
		c.Orientation = d.Orientation
	}
	switch c.Compositor {
	case CompositorAuto, CompositorWeston, CompositorCage:
	default:
		c.Compositor = d.Compositor
	}
}
