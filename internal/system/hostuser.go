package system

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
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
	if display == "" {
		display = ":0" // sudo usually strips DISPLAY; the local session is :0
	}

	xauth := os.Getenv("XAUTHORITY")
	if name != "" || xauth == "" {
		// Prefer the invoking user's cookie, not root's.
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
