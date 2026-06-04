package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// podmanCmd runs podman rootless. Binder access works because Load() sets
// /dev/binder permissions to 0666 at module-load time — no sudo needed here.
func podmanCmd(args ...string) *exec.Cmd {
	return exec.Command("podman", args...)
}

// PodmanInstalled returns true if podman is found on PATH.
func PodmanInstalled() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// ImageExistsChecked returns (exists, ok). ok is false if the check itself failed.
func ImageExistsChecked(image string) (exists, ok bool) {
	out, err := podmanCmd("images", "-q", image).Output()
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(out)) != "", true
}

// instanceDataDir returns the persistent data directory for a named instance.
func instanceDataDir(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "x11droid", "instances", name)
}

func List() ([]Instance, error) {
	out, err := podmanCmd("ps", "-a",
		"--filter", "label=x11droid=true",
		"--format", "json",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}

	s := strings.TrimSpace(string(out))
	if s == "" || s == "null" || s == "[]" {
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

// SpawnOpts holds the configuration for a new instance.
type SpawnOpts struct {
	Name    string
	GApps   bool
	HideARM bool // install libndk ARM translation layer
	PV      bool // use persistent volume for waydroid data
}

func Spawn(opts SpawnOpts) error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	xdgRuntime := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntime == "" {
		xdgRuntime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		home, _ := os.UserHomeDir()
		xauth = filepath.Join(home, ".Xauthority")
	}

	args := []string{
		"run", "-d",
		"--name", opts.Name,
		"--label", "x11droid=true",
		"--privileged",
		"--device", "/dev/binder",
		"--network=host",
		"-e", fmt.Sprintf("DISPLAY=%s", display),
		"-e", fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntime),
		"-e", "WAYLAND_DISPLAY=wayland-0",
		"-e", "WLR_BACKENDS=x11",
		"-e", "WLR_RENDERER=pixman",
		"-e", "XDG_SESSION_TYPE=x11",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		"-v", fmt.Sprintf("%s:%s", xdgRuntime, xdgRuntime),
	}

	if opts.PV {
		dataDir := instanceDataDir(opts.Name)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/var/lib/waydroid", dataDir))
	}

	if _, err := os.Stat(xauth); err == nil {
		args = append(args,
			"-e", fmt.Sprintf("XAUTHORITY=%s", xauth),
			"-v", fmt.Sprintf("%s:%s:ro", xauth, xauth),
		)
	}

	if opts.GApps {
		args = append(args, "-e", "WAYDROID_GAPPS=1")
	}
	if opts.HideARM {
		args = append(args, "-e", "WAYDROID_HIDEARM=1")
	}
	args = append(args, "x11droid:latest")

	out, err := podmanCmd(args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman run: %w\n%s", err, out)
	}
	return nil
}

func Start(name string) error {
	out, err := podmanCmd("start", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman start %s: %w\n%s", name, err, out)
	}
	return nil
}

func Stop(name string) error {
	out, err := podmanCmd("stop", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman stop %s: %w\n%s", name, err, out)
	}
	return nil
}

func Remove(name string) error {
	out, err := podmanCmd("rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman rm %s: %w\n%s", name, err, out)
	}
	return nil
}

func Logs(name string) (string, error) {
	out, err := podmanCmd("logs", "--tail", "200", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman logs %s: %w", name, err)
	}
	return string(out), nil
}

func ImageExists(image string) bool {
	out, err := podmanCmd("images", "-q", image).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// isSudoAuthErr returns true when the command failed because sudo credentials
// are not cached — expected when the user hasn't authenticated yet.
func isSudoAuthErr(out []byte) bool {
	s := strings.ToLower(string(out))
	return strings.Contains(s, "sudo") && (strings.Contains(s, "password is required") ||
		strings.Contains(s, "no password") ||
		strings.Contains(s, "a password"))
}
