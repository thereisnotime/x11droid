package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Instance struct {
	Name   string
	ID     string
	Status string
	Image  string
}

type podmanPS struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Status string   `json:"Status"`
	Image  string   `json:"Image"`
}

func List() ([]Instance, error) {
	out, err := exec.Command("podman", "ps", "-a",
		"--filter", "label=x11droid=true",
		"--format", "json",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}

	if strings.TrimSpace(string(out)) == "" || strings.TrimSpace(string(out)) == "null" {
		return nil, nil
	}

	var raw []podmanPS
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse podman output: %w", err)
	}

	instances := make([]Instance, 0, len(raw))
	for _, r := range raw {
		name := r.ID[:12]
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		instances = append(instances, Instance{
			Name:   name,
			ID:     r.ID[:12],
			Status: r.Status,
			Image:  r.Image,
		})
	}
	return instances, nil
}

func Spawn(name string, gapps bool) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	xdgRuntime := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntime == "" {
		xdgRuntime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "x11droid=true",
		"--privileged",
		"--device", "/dev/binder",
		"-e", fmt.Sprintf("DISPLAY=%s", display),
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		"-v", fmt.Sprintf("%s:%s", xdgRuntime, xdgRuntime),
		"-e", fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntime),
		"-e", "WAYLAND_DISPLAY=wayland-0",
	}
	if gapps {
		args = append(args, "-e", "WAYDROID_GAPPS=1")
	}
	args = append(args, "x11droid:latest", "/usr/bin/waydroid-session.sh")

	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Start(name string) error {
	out, err := exec.Command("podman", "start", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman start %s: %w\n%s", name, err, out)
	}
	return nil
}

func Stop(name string) error {
	out, err := exec.Command("podman", "stop", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman stop %s: %w\n%s", name, err, out)
	}
	return nil
}

func Remove(name string) error {
	out, err := exec.Command("podman", "rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman rm %s: %w\n%s", name, err, out)
	}
	return nil
}

func Exec(name, command string) error {
	cmd := exec.Command("podman", "exec", "-it", name, "bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Logs(name string) (string, error) {
	out, err := exec.Command("podman", "logs", "--tail", "100", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman logs %s: %w", name, err)
	}
	return string(out), nil
}

func ImageExists(image string) bool {
	out, err := exec.Command("podman", "images", "-q", image).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
