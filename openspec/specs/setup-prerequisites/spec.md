# Setup and Prerequisites Specification

## Purpose
Define the host-side prerequisites x11droid checks and provisions before instances can run: a working rootful Podman (x11droid runs under sudo as root), the `x11droid:latest` container image built from an embedded self-contained build context, the required kernel modules, and a local X11 session for display forwarding. The `setup` command and TUI setup view surface and remediate these.

## Requirements

### Requirement: Run rootful via sudo
The system SHALL run as root (via `sudo x11droid`) so that every podman call is rootful, because Waydroid loop-mounts the Android system image and rootless podman cannot associate a loop device even with `--privileged`.

#### Scenario: podman invoked directly as root
- **WHEN** the system runs a podman operation
- **THEN** it invokes the `podman` CLI directly (no rootless remapping), relying on the process already being root

#### Scenario: Resolving the invoking user under sudo
- **WHEN** x11droid runs under sudo and needs the user's X session and home
- **THEN** it resolves `SUDO_USER`/`SUDO_UID` to find the invoking user's home, display, X cookie, and runtime dir rather than using root's

### Requirement: Report setup status
The `setup` command SHALL report the status of podman, the kernel modules, and the `x11droid:latest` image.

#### Scenario: Status output
- **WHEN** the user runs `x11droid setup status`
- **THEN** the system prints whether podman is present, the state of each kernel module, and whether `x11droid:latest` is built

### Requirement: Build the image from an embedded self-contained context
The system SHALL build `x11droid:latest` from an embedded Containerfile and entrypoint written to the user's config dir on demand, so the binary is self-contained, and SHALL keep the slow Waydroid layer separate from the faster display-stack layer to preserve the multi-GB cache.

#### Scenario: Building the image
- **WHEN** the user runs `x11droid setup build`
- **THEN** the system writes the Containerfile and entrypoint to the config dir and runs `podman build -t x11droid:latest <dir>`

#### Scenario: Layer ordering preserves cache
- **WHEN** the image is built
- **THEN** the Waydroid install is in an earlier layer than the display stack so display-stack changes do not bust the Waydroid cache

### Requirement: Ship a fake modprobe and current entrypoint at spawn time
The system SHALL mount a no-op `modprobe` and the current entrypoint into each spawned container, so Waydroid init succeeds (binder is already loaded on the host) and entrypoint changes take effect on respawn without rebuilding the multi-GB image.

#### Scenario: Fake modprobe mounted
- **WHEN** an instance is spawned
- **THEN** a no-op `modprobe` script is mounted over `/usr/local/bin/modprobe`

#### Scenario: Current entrypoint mounted
- **WHEN** an instance is spawned
- **THEN** the current entrypoint is written to the config dir and mounted over the in-image session script

### Requirement: Provide module load/unload via setup
The system SHALL load and unload the required kernel modules through the `setup` subcommands and the TUI setup menu.

#### Scenario: Loading modules via setup
- **WHEN** the user runs `x11droid setup load`
- **THEN** the system ensures `binder_linux` is loaded

#### Scenario: Unloading modules via setup
- **WHEN** the user runs `x11droid setup unload`
- **THEN** the system unloads the tracked modules

### Requirement: Detect the host display session
The system SHALL detect whether the host session is X11, Wayland, XWayland, or unknown, and SHALL warn when the session is not plain X11 because display forwarding requires a local X11 server.

#### Scenario: Plain X11 session
- **WHEN** the session is X11
- **THEN** no display warning is emitted

#### Scenario: Wayland session warning
- **WHEN** the session is Wayland
- **THEN** the system warns that display forwarding requires X11 and the compositor cannot connect to a Wayland compositor

#### Scenario: Missing display warning
- **WHEN** the session type is unknown and `DISPLAY` is unset
- **THEN** the system warns that there is no X11 server to connect to
