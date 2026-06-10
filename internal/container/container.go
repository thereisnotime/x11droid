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
	DeviceName string // Android device/model name (empty = use instance name)
	GApps      bool
	HideARM    bool   // install libndk ARM translation layer
	FDroid     bool   // install F-Droid after first boot
	Aurora     bool   // install Aurora Store after first boot
	Obtainium  bool   // install Obtainium after first boot
	Shelter    bool   // install Shelter after first boot
	DevOptions bool   // enable Android Developer Options on first boot
	Root       bool   // install Magisk (root) on first boot
	PV         bool   // use persistent volume for waydroid data
	Width      int    // compositor window width  (0 = image default)
	Height     int    // compositor window height (0 = image default)
	Compositor string // "", "auto", "weston" or "cage"
}

// selectedApps returns the comma-separated WAYDROID_APPS value for the apps the
// user picked, or "" when none are selected (so the env var is omitted).
func selectedApps(opts SpawnOpts) string {
	var apps []string
	if opts.FDroid {
		apps = append(apps, "fdroid")
	}
	if opts.Aurora {
		apps = append(apps, "aurora")
	}
	if opts.Obtainium {
		apps = append(apps, "obtainium")
	}
	if opts.Shelter {
		apps = append(apps, "shelter")
	}
	return strings.Join(apps, ",")
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
		// Share the host cgroup namespace so the Android LXC can delegate
		// cgroup controllers — without this its init fails to enable
		// controllers ("Device or resource busy") and Android boot-loops.
		"--cgroupns=host",
		// No PID limit. Podman defaults to 2048; Android (especially with
		// GApps/GMS) spawns more threads than that, and the overflow surfaces
		// as "pthread_create failed: Try again" → system_server dies → reboot.
		"--pids-limit=-1",
		// Name available to the entrypoint for the weston window title.
		"-e", fmt.Sprintf("X11DROID_NAME=%s", opts.Name),
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
	if opts.DevOptions {
		args = append(args, "-e", "WAYDROID_DEVOPTS=1")
	}
	if opts.Root {
		args = append(args, "-e", "WAYDROID_ROOT=1")
	}
	if apps := selectedApps(opts); apps != "" {
		args = append(args, "-e", fmt.Sprintf("WAYDROID_APPS=%s", apps))
	}
	if opts.DeviceName != "" {
		args = append(args, "-e", fmt.Sprintf("WAYDROID_DEVICE=%s", opts.DeviceName))
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
	// Give waydroid a short grace period to unmount the LXC, then force it —
	// the default 10s leaves the container in "Stopping" too long.
	out, err := podmanCmd("stop", "-t", "5", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman stop %s: %w\n%s", name, err, out)
	}
	return nil
}

func Remove(name string) error {
	// -t 0 force-kills immediately — waydroid containers can hang in "Stopping"
	// (loop mounts + LXC not tearing down), and a graceful wait never returns.
	out, err := podmanCmd("rm", "-f", "-t", "0", name).CombinedOutput()
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
	// Show-full-ui renders into the running compositor; if it died (e.g. the
	// window was closed) there's nothing to show into, so fail with a clear
	// message instead of silently doing nothing. Re-point the fresh exec env at
	// the entrypoint's session bus and the compositor's wayland socket, then
	// background show-full-ui (it blocks while the UI is open) via setsid so it
	// survives the exec returning.
	script := `pgrep -x weston >/dev/null 2>&1 || pgrep -x cage >/dev/null 2>&1 || ` +
		`{ echo "compositor is not running — Stop then Start (or respawn) this instance"; exit 1; }
export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/dbus/session_bus_socket"
export WAYLAND_DISPLAY="$(basename "$(ls "$XDG_RUNTIME_DIR"/wayland-[0-9]* 2>/dev/null | head -1)")"
setsid waydroid show-full-ui >/dev/null 2>&1 </dev/null &
sleep 1`
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
