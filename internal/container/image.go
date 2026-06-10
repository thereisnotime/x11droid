package container

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/thereisnotime/x11droid/internal/system"
)

// entrypointContent is the container entrypoint script, embedded so the
// app-driven build is self-contained. The repo Containerfile copies the same
// file directly; this keeps a single source of truth.
//
//go:embed entrypoint.sh
var entrypointContent string

// containerfileContent is the embedded Containerfile written to
// ~/.config/x11droid/ on demand (alongside entrypoint.sh) so the binary is
// self-contained.
const containerfileContent = `FROM ubuntu:24.04

# Only apt-related env here — changing anything above the waydroid layer
# busts its cache and triggers a multi-GB re-download.
ENV DEBIAN_FRONTEND=noninteractive

# Layer 1 — waydroid (slow, changes rarely — must stay cached)
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    curl https://repo.waydro.id | bash && \
    apt-get install -y --no-install-recommends waydroid && \
    rm -rf /var/lib/apt/lists/*

# Layer 2 — display stack (faster, safe to modify without busting waydroid cache)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3-pip \
        wl-clipboard \
        dbus \
        cage \
        weston && \
    rm -rf /var/lib/apt/lists/*

# Runtime env — after all slow layers so changing these doesn't bust cache
ENV WLR_BACKENDS=x11 \
    WLR_RENDERER=pixman \
    XDG_SESSION_TYPE=x11

RUN printf '#!/bin/sh\nexec true\n' > /usr/local/bin/modprobe && \
    chmod +x /usr/local/bin/modprobe

COPY entrypoint.sh /usr/bin/waydroid-session.sh
RUN chmod +x /usr/bin/waydroid-session.sh

ENTRYPOINT ["/usr/bin/waydroid-session.sh"]
`

func configDir() string {
	home := system.ResolveHostUser().Home
	if home == "" {
		return "/tmp/x11droid"
	}
	return filepath.Join(home, ".config", "x11droid")
}

// ensureFakeModprobe writes a no-op modprobe script so waydroid init succeeds
// inside the container — binder is already loaded on the host.
func ensureFakeModprobe(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	return os.WriteFile(path, []byte("#!/bin/sh\nexec true\n"), 0755)
}

// ensureContainerfile writes the embedded Containerfile and entrypoint.sh to
// ~/.config/x11droid/ so the directory is a self-contained build context.
func ensureContainerfile() (string, error) {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Containerfile"), []byte(containerfileContent), 0644); err != nil {
		return "", fmt.Errorf("write Containerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entrypoint.sh"), []byte(entrypointContent), 0755); err != nil {
		return "", fmt.Errorf("write entrypoint.sh: %w", err)
	}
	return dir, nil
}

// BuildImageCmd returns the podman build command ready to be handed to
// tea.ExecProcess so the TUI suspends and build output is fully visible.
func BuildImageCmd() (*exec.Cmd, error) {
	dir, err := ensureContainerfile()
	if err != nil {
		return nil, err
	}
	return exec.Command("podman", "build", "-t", "x11droid:latest", dir), nil
}
