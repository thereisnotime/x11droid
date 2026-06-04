package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// containerfileContent is the embedded Containerfile. It is written to
// ~/.config/x11droid/ on demand so the binary is self-contained.
const containerfileContent = `FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    WLR_BACKENDS=x11 \
    WLR_RENDERER=pixman \
    XDG_SESSION_TYPE=x11

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        python3-pip \
        wl-clipboard \
        cage \
        weston \
        kmod && \
    rm -rf /var/lib/apt/lists/*

RUN curl https://repo.waydro.id | bash && \
    apt-get install -y --no-install-recommends waydroid && \
    rm -rf /var/lib/apt/lists/*

RUN printf '#!/bin/bash\n\
COMPOSITOR="${WAYDROID_COMPOSITOR:-cage}"\n\
GAPPS="${WAYDROID_GAPPS:-}"\n\
\n\
cleanup() {\n\
  trap - EXIT INT TERM HUP\n\
  waydroid session stop 2>/dev/null || true\n\
  killall waydroid cage weston 2>/dev/null || true\n\
}\n\
trap cleanup EXIT INT TERM HUP\n\
\n\
# First-run initialisation — downloads Android image (~500MB).\n\
if [ ! -f /var/lib/waydroid/images/system.img ]; then\n\
  echo "[x11droid] First run: initialising Waydroid (this downloads ~500MB, please wait)..."\n\
  if [ -n "$GAPPS" ]; then\n\
    waydroid init -f -s GAPPS\n\
  else\n\
    waydroid init -f\n\
  fi\n\
  if [ $? -ne 0 ]; then\n\
    echo "[x11droid] waydroid init failed — check logs with: x11droid logs <name>"\n\
    exit 1\n\
  fi\n\
  echo "[x11droid] Init done, starting UI..."\n\
fi\n\
\n\
case "$COMPOSITOR" in\n\
  cage)\n\
    cage -s -- waydroid show-full-ui\n\
    ;;\n\
  weston)\n\
    weston --xwayland &\n\
    export WAYLAND_DISPLAY=wayland-0\n\
    sleep 2\n\
    waydroid show-full-ui\n\
    ;;\n\
  *)\n\
    echo "Unknown compositor: $COMPOSITOR" >&2\n\
    exit 1\n\
    ;;\n\
esac\n' > /usr/bin/waydroid-session.sh && \
    chmod +x /usr/bin/waydroid-session.sh

ENTRYPOINT ["/usr/bin/waydroid-session.sh"]
`

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/x11droid"
	}
	return filepath.Join(home, ".config", "x11droid")
}

// ensureContainerfile writes the embedded Containerfile to
// ~/.config/x11droid/Containerfile if it does not already exist.
func ensureContainerfile() (string, error) {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	dst := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(dst, []byte(containerfileContent), 0644); err != nil {
		return "", fmt.Errorf("write Containerfile: %w", err)
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
