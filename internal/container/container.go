package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thereisnotime/x11droid/internal/kernel"
	"github.com/thereisnotime/x11droid/internal/system"
)

type Instance struct {
	Name    string
	ID      string
	Status  string
	Image   string
	Created string // formatted creation time
	RAM     string // memory used (running only; "-" otherwise)
}

type podmanPS struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Status  string   `json:"Status"`
	Image   string   `json:"Image"`
	Created int64    `json:"Created"`
}

// Extras is per-instance info fetched on demand for the detail view (touches
// the filesystem and runs `podman stats`, so it's slower than List).
type Extras struct {
	DataDir    string
	Persistent bool // data dir exists on disk
	Size       string
	MemUsage   string // current RAM (used / limit), empty if not running
	LibNDK     bool   // ARM translation installed
	Magisk     bool   // root installed
	Apps       []string
}

// InstanceExtras gathers the data-dir path/size, current RAM usage, and which
// one-time system mods are installed (marker files the entrypoint writes).
func InstanceExtras(name string) Extras {
	dir := instanceDataDir(name)
	e := Extras{DataDir: dir, MemUsage: memUsage(name)}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return e
	}
	e.Persistent = true
	e.Size = dirSize(dir)
	e.LibNDK = fileExists(filepath.Join(dir, ".x11droid-libndk"))
	e.Magisk = fileExists(filepath.Join(dir, ".x11droid-magisk"))
	e.Apps = readLines(filepath.Join(dir, ".x11droid-apps"))
	return e
}

// readLines returns the non-empty trimmed lines of a file (the installed-app
// names the entrypoint records), or nil.
func readLines(p string) []string {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// memUsage returns the container's current memory usage ("used / limit"), or
// "" when it isn't running.
func memUsage(name string) string {
	out, err := podmanCmd("stats", "--no-stream", "--format", "{{.MemUsage}}", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirSize(dir string) string {
	out, err := exec.Command("du", "-sh", dir).Output()
	if err != nil {
		return ""
	}
	if f := strings.Fields(string(out)); len(f) > 0 {
		return f[0]
	}
	return ""
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

// instancesRoot is the directory holding every instance's persistent data.
func instancesRoot() string {
	return filepath.Join(system.ResolveHostUser().Home, ".config", "x11droid", "instances")
}

// instanceDataDir returns the persistent data directory for a named instance,
// kept under the invoking user's home (not root's) so data is consistent.
func instanceDataDir(name string) string {
	return filepath.Join(instancesRoot(), name)
}

// DataDir describes one instance's on-disk data and whether a container still
// exists for it (a dir without a container is an orphan, safe to prune).
type DataDir struct {
	Name         string
	Path         string
	Size         string
	HasContainer bool
}

// DataDirs lists every instance data directory with its size and whether a
// matching container exists.
func DataDirs() ([]DataDir, error) {
	entries, err := os.ReadDir(instancesRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	have := map[string]bool{}
	if instances, err := List(); err == nil {
		for _, i := range instances {
			have[i.Name] = true
		}
	}
	var out []DataDir
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(instancesRoot(), e.Name())
		out = append(out, DataDir{
			Name:         e.Name(),
			Path:         p,
			Size:         dirSize(p),
			HasContainer: have[e.Name()],
		})
	}
	return out, nil
}

// PruneOrphans deletes data dirs that have no container and returns their names.
func PruneOrphans() ([]string, error) {
	dds, err := DataDirs()
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, d := range dds {
		if d.HasContainer {
			continue
		}
		if err := os.RemoveAll(d.Path); err != nil {
			return removed, fmt.Errorf("remove %s: %w", d.Path, err)
		}
		removed = append(removed, d.Name)
	}
	return removed, nil
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

	stats := memStats() // one batched stats call for all running containers

	instances := make([]Instance, 0, len(raw))
	for _, r := range raw {
		name := r.ID[:12]
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		created := ""
		if r.Created > 0 {
			created = time.Unix(r.Created, 0).Format("2006-01-02 15:04")
		}
		ram := "-"
		if v, ok := stats[name]; ok && v != "" {
			// keep just the "used" half of "used / limit" for a compact column
			ram = strings.TrimSpace(strings.SplitN(v, "/", 2)[0])
		}
		instances = append(instances, Instance{
			Name:    name,
			ID:      r.ID[:12],
			Status:  r.Status,
			Image:   r.Image,
			Created: created,
			RAM:     ram,
		})
	}
	return instances, nil
}

// memStats returns a name→memory-usage map for all running containers from a
// single `podman stats` call — cheaper than one call per instance when
// building the dashboard list. Stopped containers simply won't appear.
func memStats() map[string]string {
	out, err := podmanCmd("stats", "--no-stream", "--format", "{{.Name}}\t{{.MemUsage}}").Output()
	if err != nil {
		return nil
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
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
		// Keep the host runtime dir (waydroid's LXC bind-mounts its pulse
		// socket); weston uses a per-instance socket name (entrypoint) so two
		// instances don't collide on the shared dir.
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

	// Mount the current entrypoint over the baked-in copy so it ships with the
	// binary — changes take effect on respawn, no multi-GB image rebuild needed.
	entrypoint := filepath.Join(configDir(), "entrypoint.sh")
	if err := os.MkdirAll(configDir(), 0755); err == nil {
		if err := os.WriteFile(entrypoint, []byte(entrypointContent), 0755); err == nil {
			args = append(args, "-v", fmt.Sprintf("%s:/usr/bin/waydroid-session.sh:ro", entrypoint))
		}
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
	// Show-full-ui renders into the running compositor. weston is a background
	// child of the entrypoint with nothing watching it, so if it died (window
	// closed, X hiccup, resource contention with another instance) the container
	// stays Up but has no display and show-full-ui has nothing to render into.
	// Rather than dead-ending, relaunch weston first — reusing the original
	// DISPLAY/XAUTHORITY/geometry from the entrypoint's environment (PID 1's
	// /proc environ) — so "Show UI" doubles as a recovery action.
	// weston runs with --socket=wl-<name> (per-instance); use that exact name —
	// globbing the shared XDG_RUNTIME_DIR could grab another instance's socket.
	sock := "wl-" + name
	title := "x11droid - " + name + " - weston"
	// Identify the window by the id weston writes to /tmp/weston.log, not by
	// title: a compositor relaunched here doesn't run the entrypoint's titler,
	// so its window has weston's default name. The log line ("window id N") is
	// the same source the entrypoint's titler uses and is always present.
	script := `sock="` + sock + `"
title="` + title + `"
eval "$(tr '\0' '\n' </proc/1/environ | grep -E '^(DISPLAY|XAUTHORITY|XDG_RUNTIME_DIR|WAYDROID_WIDTH|WAYDROID_HEIGHT)=' | sed 's/^/export /')"
getwid() { grep -oE 'window id [0-9]+' /tmp/weston.log 2>/dev/null | head -1 | awk '{print $NF}'; }
if ! pgrep -x weston >/dev/null 2>&1 && ! pgrep -x cage >/dev/null 2>&1; then
  # Compositor died — relaunch it with the same display/geometry the entrypoint used.
  W="${WAYDROID_WIDTH:-540}"; H="${WAYDROID_HEIGHT:-960}"
  weston --backend=x11-backend.so --use-pixman --shell=kiosk-shell.so \
    --socket="$sock" --width="$W" --height="$H" >/tmp/weston.log 2>&1 &
  for _ in $(seq 1 30); do [ -S "$XDG_RUNTIME_DIR/$sock" ] && break; sleep 0.5; done
  [ -S "$XDG_RUNTIME_DIR/$sock" ] || { echo "compositor failed to relaunch:"; tail -5 /tmp/weston.log; exit 1; }
  # Title the fresh window so Hide UI (and the user) can recognise it.
  for _ in $(seq 1 20); do [ -n "$(getwid)" ] && break; sleep 0.3; done
  wid="$(getwid)"; [ -n "$wid" ] && xdotool set_window --name "$title" "$wid" 2>/dev/null || true
  export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/dbus/session_bus_socket"
  export WAYLAND_DISPLAY="$sock"
  setsid waydroid show-full-ui >/dev/null 2>&1 </dev/null &
  sleep 1
else
  # Compositor alive — Hide UI just unmapped the window; re-map and raise it.
  wid="$(getwid)"
  if [ -n "$wid" ]; then
    xdotool windowmap "$wid" 2>/dev/null || true
    xdotool windowactivate "$wid" 2>/dev/null || true
  fi
fi`
	out, err := podmanCmd("exec", name, "bash", "-lc", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("show-full-ui %s: %w\n%s", name, err, out)
	}
	return nil
}

// HideUI hides the Android window without touching Android: it just unmaps the
// weston output window on the host X server. weston and the Android session keep
// running, so ShowUI re-maps it instantly — no session stop, no black screen, no
// reboot. (ShowUI still relaunches the compositor if it actually died.)
func HideUI(name string) error {
	// Match the window by the id weston logged, not its title (a Show-UI-relaunched
	// compositor has no x11droid title).
	script := `eval "$(tr '\0' '\n' </proc/1/environ | grep -E '^(DISPLAY|XAUTHORITY)=' | sed 's/^/export /')"
wid="$(grep -oE 'window id [0-9]+' /tmp/weston.log 2>/dev/null | head -1 | awk '{print $NF}')"
[ -n "$wid" ] || { echo "no compositor window found for ` + name + ` (is the UI shown?)"; exit 1; }
xdotool windowunmap "$wid" 2>/dev/null || true`
	out, err := podmanCmd("exec", name, "bash", "-lc", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("hide-ui %s: %w\n%s", name, err, out)
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
