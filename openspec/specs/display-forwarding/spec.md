# Display Forwarding Specification

## Purpose
Define how an instance's Android UI is displayed on the host's X11 session. A nested Wayland compositor (weston with the pixman renderer, or cage when supported) runs inside the container and forwards its single fullscreen output to the host X server over the mounted `/tmp/.X11-unix` socket. The system also supports showing, hiding, and recovering the per-instance window.

## Requirements

### Requirement: Forward the compositor output to the host X server
The container SHALL render the Android UI through a nested compositor that connects to the host X11 display, using the host `DISPLAY` and the mounted `/tmp/.X11-unix` socket rather than the network.

#### Scenario: X11 socket and display passed to the container
- **WHEN** an instance is spawned
- **THEN** the run arguments mount `/tmp/.X11-unix:/tmp/.X11-unix` and set `DISPLAY` to the host user's display

#### Scenario: wlroots backend forced to X11/pixman
- **WHEN** an instance is spawned
- **THEN** the environment sets `WLR_BACKENDS=x11`, `WLR_RENDERER=pixman`, and `XDG_SESSION_TYPE=x11`

### Requirement: Default to weston with pixman, with cage probing under auto
The compositor SHALL default to `auto`, which probes cage and falls back to weston when cage cannot allocate buffers (e.g. on NVIDIA); `weston` and `cage` MAY be forced explicitly. weston runs with the x11 backend, `--use-pixman`, and the kiosk shell so the Android window fills the output.

#### Scenario: auto falls back to weston on NVIDIA
- **WHEN** the compositor is `auto` and a quick `cage` probe fails
- **THEN** the entrypoint launches weston with `--backend=x11-backend.so --use-pixman --shell=kiosk-shell.so` instead of cage

#### Scenario: auto uses cage when supported
- **WHEN** the compositor is `auto` and the `cage` probe succeeds
- **THEN** the entrypoint launches cage

#### Scenario: Forcing a compositor
- **WHEN** the user sets the compositor to `weston` or `cage`
- **THEN** the entrypoint launches that compositor without probing

### Requirement: Use a per-instance Wayland socket
Each instance's compositor SHALL bind a unique Wayland socket named `wl-<name>` so multiple instances sharing the host `XDG_RUNTIME_DIR` do not collide on the same compositor.

#### Scenario: Unique socket per instance
- **WHEN** weston starts for an instance named `phone`
- **THEN** it runs with `--socket=wl-phone` and `WAYLAND_DISPLAY` is set to `wl-phone`

### Requirement: Authenticate the container to the host X server
The system SHALL provide the container an X11 authorization cookie, preferring a FamilyWild (hostname-agnostic) copy so the rootful container — with a different hostname, connecting as real root — still authenticates, falling back to the raw cookie when `xauth` is unavailable.

#### Scenario: Wild cookie generated and mounted
- **WHEN** an instance is spawned and `xauth` is available
- **THEN** a FamilyWild copy of the display's cookie is created and mounted into the container, with `XAUTHORITY` pointed at it

#### Scenario: Fallback to raw cookie
- **WHEN** `xauth` is unavailable or the wild cookie cannot be produced
- **THEN** the raw `XAUTHORITY` cookie is used instead

### Requirement: Show or recover the Android window
The system SHALL (re)open the Android window for a running instance by invoking `waydroid show-full-ui` on the in-container compositor, and SHALL relaunch the compositor first if it died (so "Show UI" doubles as a recovery action), reusing the original display/geometry from PID 1's environment.

#### Scenario: Re-mapping an alive compositor
- **WHEN** Show UI runs and a weston/cage process is already alive
- **THEN** the system re-maps and raises the existing window via xdotool rather than relaunching

#### Scenario: Recovering a dead compositor
- **WHEN** Show UI runs and no compositor process is alive
- **THEN** the system relaunches weston with the saved socket/geometry, waits for its socket, titles its window, and starts `waydroid show-full-ui`

### Requirement: Hide the window without stopping Android
The system SHALL hide an instance's window by unmapping the weston output window on the host X server, leaving weston and the Android session running so the window can be re-shown instantly.

#### Scenario: Hiding a window
- **WHEN** the user hides an instance's UI
- **THEN** the system unmaps the compositor window via xdotool and does not stop Android or the container

### Requirement: Identify the window by the weston log id
The system SHALL locate an instance's X11 window by the window id weston writes to `/tmp/weston.log` (not by title), so a compositor relaunched during recovery — which lacks the entrypoint's custom title — can still be matched.

#### Scenario: Window matched by logged id
- **WHEN** Show UI or Hide UI needs the window id
- **THEN** it parses `window id N` from `/tmp/weston.log` to find the window
