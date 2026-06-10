package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thereisnotime/x11droid/internal/kernel"
	"github.com/thereisnotime/x11droid/internal/system"
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

// podmanCmd runs the podman CLI directly. x11droid itself must run as root
// (`sudo x11droid`) because waydroid requires rootful podman — a rootless
// container cannot associate a loop device (needs real host CAP_SYS_ADMIN) to
// mount the Android system.img. Running as root makes every podman call rootful.
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

// instanceDataDir returns the persistent data directory for a named instance,
// kept under the invoking user's home (not root's) so data is consistent.
func instanceDataDir(name string) string {
	home := system.ResolveHostUser().Home
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
	Name       string
	GApps      bool
	HideARM    bool   // install libndk ARM translation layer
	PV         bool   // use persistent volume for waydroid data
	Width      int    // compositor window width  (0 = image default)
	Height     int    // compositor window height (0 = image default)
	Compositor string // "", "auto", "weston" or "cage"
}

func Spawn(opts SpawnOpts) error {
	// Resolve the invoking user's X session — under sudo the process env is
	// root's, so DISPLAY/XAUTHORITY/XDG_RUNTIME_DIR must come from SUDO_USER.
	hu := system.ResolveHostUser()
	display := hu.Display
	xdgRuntime := hu.RuntimeDir
	xauth := hu.XAuthority

	args := []string{
		"run", "-d",
		"--name", opts.Name,
		"--label", "x11droid=true",
		"--privileged",
		// NOT --network=host: waydroid needs its own netns to create the
		// waydroid0 bridge (rootless can't modify the host netns). X11 reaches
		// the host over the mounted /tmp/.X11-unix socket, not the network.
		// Share the host IPC namespace so the compositor's MIT-SHM / pixman
		// buffers can be shared with the host X server.
		"--ipc=host",
		"-e", fmt.Sprintf("DISPLAY=%s", display),
		"-e", fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntime),
		"-e", "WLR_BACKENDS=x11",
		"-e", "WLR_RENDERER=pixman",
		"-e", "XDG_SESSION_TYPE=x11",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		"-v", fmt.Sprintf("%s:%s", xdgRuntime, xdgRuntime),
	}

	// Pass each binder node waydroid needs (binder, hwbinder, vndbinder). These
	// only exist if the module was loaded with the devices parameter — see
	// kernel.Load(). Skip any that are missing rather than failing the run.
	for _, dev := range kernel.BinderDeviceNodes() {
		if _, err := os.Stat(dev); err == nil {
			args = append(args, "--device", dev)
		}
	}

	// Hand the GPU render node to the container when present — cage's GBM path
	// needs it on mesa systems; weston's pixman path ignores it harmlessly.
	if _, err := os.Stat("/dev/dri"); err == nil {
		args = append(args, "--device", "/dev/dri")
	}

	if opts.Width > 0 {
		args = append(args, "-e", fmt.Sprintf("WAYDROID_WIDTH=%d", opts.Width))
	}
	if opts.Height > 0 {
		args = append(args, "-e", fmt.Sprintf("WAYDROID_HEIGHT=%d", opts.Height))
	}
	if opts.Compositor != "" {
		args = append(args, "-e", fmt.Sprintf("WAYDROID_COMPOSITOR=%s", opts.Compositor))
	}

	// Inject a fake modprobe so waydroid init doesn't fail looking for it —
	// binder is already loaded on the host so modprobe just needs to succeed.
	fakeModprobe := filepath.Join(configDir(), "bin", "modprobe")
	if err := ensureFakeModprobe(fakeModprobe); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/usr/local/bin/modprobe:ro", fakeModprobe))
	}

	if opts.PV {
		dataDir := instanceDataDir(opts.Name)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/var/lib/waydroid", dataDir))
	}

	// Give the container an X11 cookie. Use a hostname-agnostic (FamilyWild)
	// copy so the rootful container — different hostname, connecting as real
	// root — still authenticates to the host X server; fall back to the raw
	// cookie if xauth isn't available.
	cookie := wildXauth(xauth, display)
	if cookie == "" {
		cookie = xauth
	}
	if _, err := os.Stat(cookie); err == nil {
		args = append(args,
			"-e", "XAUTHORITY=/tmp/.x11droid.xauth",
			"-v", fmt.Sprintf("%s:/tmp/.x11droid.xauth:ro", cookie),
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

// wildXauth writes a FamilyWild (hostname-agnostic) copy of the X11 cookie for
// the given display and returns its path. This lets a container with a
// different hostname, connecting as real root under rootful podman, still
// authenticate to the host X server. Returns "" if xauth is unavailable or the
// cookie can't be extracted.
func wildXauth(srcXauth, display string) string {
	if _, err := exec.LookPath("xauth"); err != nil {
		return ""
	}
	dst := filepath.Join(configDir(), "xauth")
	_ = os.Remove(dst)
	// Extract the cookie for this display, rewrite its address family to wild
	// (ffff), and merge it into a fresh file.
	script := fmt.Sprintf(
		"xauth -f %q nlist %q 2>/dev/null | sed -e 's/^..../ffff/' | xauth -f %q nmerge - 2>/dev/null",
		srcXauth, display, dst,
	)
	if err := exec.Command("sh", "-c", script).Run(); err != nil {
		return ""
	}
	if fi, err := os.Stat(dst); err != nil || fi.Size() == 0 {
		return ""
	}
	return dst
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

// Purge removes the container and deletes its persistent data directory
// (the Android images + waydroid state), giving a fully clean slate. Removing
// the container is best-effort so a leftover data dir can still be cleared.
func Purge(name string) error {
	rmErr := Remove(name)
	dir := instanceDataDir(name)
	// We run as root, so we can delete the root-owned files the container wrote.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove data dir %s: %w", dir, err)
	}
	return rmErr
}

func Logs(name string) (string, error) {
	out, err := podmanCmd("logs", "--tail", "200", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman logs %s: %w", name, err)
	}
	return string(out), nil
}

// IsRunning reports whether an instance's container is currently up.
func (i Instance) IsRunning() bool {
	return strings.HasPrefix(i.Status, "Up")
}

// Running returns only the instances whose container is currently up.
func Running() ([]Instance, error) {
	all, err := List()
	if err != nil {
		return nil, err
	}
	var up []Instance
	for _, i := range all {
		if i.IsRunning() {
			up = append(up, i)
		}
	}
	return up, nil
}

// ShowUI (re)opens the Android UI window for a running instance by asking
// waydroid to show the full UI on the compositor already running inside it.
func ShowUI(name string) error {
	// A `podman exec` gets a fresh env, so re-point it at the same session bus
	// the entrypoint started (otherwise waydroid tries dbus-launch and fails)
	// and rediscover the compositor's wayland socket (weston-N or cage).
	script := `export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/dbus/session_bus_socket"; ` +
		`export WAYLAND_DISPLAY="$(basename "$(ls "$XDG_RUNTIME_DIR"/wayland-[0-9]* 2>/dev/null | head -1)")"; ` +
		`waydroid show-full-ui`
	out, err := podmanCmd("exec", name, "bash", "-lc", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("show-full-ui %s: %w\n%s", name, err, out)
	}
	return nil
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
