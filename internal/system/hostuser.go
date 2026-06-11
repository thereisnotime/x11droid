// SPDX-License-Identifier: GPL-3.0-only

package system

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// HostUser describes the human user whose X11 session and home directory the
// containers should target. When x11droid runs under sudo (as it must, for
// rootful podman), the process env belongs to root — so we resolve the
// invoking user (SUDO_USER) to find their display, auth cookie and runtime dir.
type HostUser struct {
	Name       string
	Home       string
	UID        int
	GID        int
	Display    string
	XAuthority string
	RuntimeDir string
}

// IsRoot reports whether the process is running with root privileges.
func IsRoot() bool { return os.Geteuid() == 0 }

// userXEnv finds the DISPLAY and XAUTHORITY of a live X11 process owned by uid
// by scanning /proc (readable as root). This locates the real, current session
// cookie, which display managers often keep outside ~/.Xauthority and rotate.
func userXEnv(uid int) (display, xauth string) {
	entries, _ := filepath.Glob("/proc/[0-9]*/environ")
	var fallbackD, fallbackX string
	for _, p := range entries {
		fi, err := os.Stat(filepath.Dir(p))
		if err != nil {
			continue
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || int(st.Uid) != uid {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var d, x string
		for _, kv := range strings.Split(string(data), "\x00") {
			switch {
			case strings.HasPrefix(kv, "DISPLAY="):
				d = strings.TrimPrefix(kv, "DISPLAY=")
			case strings.HasPrefix(kv, "XAUTHORITY="):
				x = strings.TrimPrefix(kv, "XAUTHORITY=")
			}
		}
		if d == "" {
			continue
		}
		// Prefer a process that gives us both, and a real local display.
		if x != "" && strings.HasPrefix(d, ":") {
			return d, x
		}
		if fallbackD == "" {
			fallbackD, fallbackX = d, x
		}
	}
	return fallbackD, fallbackX
}

// ResolveHostUser determines the target user's session parameters. Under sudo
// it uses SUDO_USER/SUDO_UID; otherwise it falls back to the current env.
func ResolveHostUser() HostUser {
	name := os.Getenv("SUDO_USER")
	uid := os.Getuid()
	gid := os.Getgid()
	var home string

	if name != "" {
		if u, err := user.Lookup(name); err == nil {
			home = u.HomeDir
			if n, err := strconv.Atoi(u.Uid); err == nil {
				uid = n
			}
			if n, err := strconv.Atoi(u.Gid); err == nil {
				gid = n
			}
		}
		// SUDO_UID/SUDO_GID are authoritative if present.
		if n, err := strconv.Atoi(os.Getenv("SUDO_UID")); err == nil {
			uid = n
		}
		if n, err := strconv.Atoi(os.Getenv("SUDO_GID")); err == nil {
			gid = n
		}
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	display := os.Getenv("DISPLAY")
	xauth := os.Getenv("XAUTHORITY")

	// Under sudo the process env is root's, so discover the invoking user's live
	// X session (DISPLAY + the *current* XAUTHORITY, which often isn't
	// ~/.Xauthority and rotates) by inspecting their processes.
	if name != "" {
		if d, x := userXEnv(uid); d != "" {
			display = d
			if x != "" {
				xauth = x
			}
		}
	}

	if display == "" {
		display = ":0" // local session fallback
	}
	if xauth == "" {
		xauth = filepath.Join(home, ".Xauthority")
	}

	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if name != "" || runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", uid)
	}

	return HostUser{
		Name:       name,
		Home:       home,
		UID:        uid,
		GID:        gid,
		Display:    display,
		XAuthority: xauth,
		RuntimeDir: runtime,
	}
}
