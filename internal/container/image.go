package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func containerfilePath() string {
	// Prefer ~/.config/x11droid/Containerfile, fall back to repo root (next to the binary).
	homeConf := filepath.Join(os.Getenv("HOME"), ".config", "x11droid", "Containerfile")
	if _, err := os.Stat(homeConf); err == nil {
		return filepath.Dir(homeConf)
	}

	// Try relative to the executable.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "Containerfile")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		// One level up (e.g. running from cmd/x11droid/ during dev).
		candidate = filepath.Join(dir, "..", "Containerfile")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(filepath.Join(dir, ".."))
		}
	}

	return "."
}

func BuildImage() error {
	dir := containerfilePath()
	fmt.Printf("Building x11droid:latest from %s ...\n", dir)
	cmd := exec.Command("podman", "build", "-t", "x11droid:latest", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman build: %w", err)
	}
	return nil
}
