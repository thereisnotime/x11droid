# x11droid

Go CLI/TUI for running and managing [Waydroid](https://waydro.id) (Android) instances inside rootless Podman containers, displayed on an X11 session. Each instance is a privileged container running a nested Wayland compositor (cage/weston) that forwards its output to the host `$DISPLAY`. Android runs natively on the host kernel via `binder` IPC — no emulation or VM.

## Layout

- `cmd/x11droid/` — entrypoint (`main.go`) + Cobra CLI (`cli.go`). Bare invocation launches the Bubble Tea TUI; subcommands (`list`, `spawn`, `start`, `stop`, `rm`, `logs`, `shell`, `setup`) are scriptable.
- `internal/container/` — Podman lifecycle (`container.go`) and the embedded `Containerfile` + image build (`image.go`). The Containerfile is split into stable (waydroid) and fast-changing (display stack) layers so the multi-GB waydroid layer stays cached — do not reorder layers carelessly.
- `internal/kernel/` — `binder_linux` (required) / `ashmem_linux` (optional, built-in on kernels ≥5.18) module load/unload. `Load()` also `chmod 0666 /dev/binder` for rootless podman.
- `internal/system/` — X11/Wayland/XWayland session detection. Display forwarding requires a local X11 session.
- `internal/tui/` — Bubble Tea TUI (model/views/keys/styles).

## Tooling — always use `just`

Use the `just` recipes for all build/test/run/setup tasks. Only fall back to raw `go`/`podman` commands when no recipe covers what you need.

- `just build` / `just run` / `just install` / `just clean`
- `just check` (vet + test + lint), or `just test` / `just vet` / `just lint` individually
- `just tidy` — go mod tidy
- `just modules-load` / `just modules-unload` / `just modules-status`
- `just image-build` / `just image-clean`
- `just setup` — modules-load + image-build (first-time)

Go is pinned via asdf (`.tool-versions`); recipes call `asdf exec go` / `asdf exec golangci-lint`.

## Conventions

- **Rootful podman** (`sudo podman`): waydroid loop-mounts the Android system image, which rootless podman cannot do (loop association needs real host CAP_SYS_ADMIN even with `--privileged`). All podman calls go through `sudo` via `podmanCmd`; authenticate once (Setup → Authenticate sudo / `x11droid setup auth`). Rootful storage is separate from rootless, so the image must be (re)built rootful.
- The container provisions binder itself via **binderfs** (mount + `BINDER_CTL_ADD` ioctl) since this kernel sets `CONFIG_ANDROID_BINDER_DEVICES=""`. It also runs its own system + session D-Bus, and uses its **own** netns (not `--network=host`) so waydroid can create the `waydroid0` bridge. See `internal/container/entrypoint.sh`.
- Display: weston (`--use-pixman`) forwards to the host X11 `:0` over the mounted `/tmp/.X11-unix` socket — cage fails on NVIDIA.
- Binary is self-contained — Containerfile, entrypoint.sh and a fake `modprobe` are written to `~/.config/x11droid/` on demand.
- Run lint and tests before considering a change done.
