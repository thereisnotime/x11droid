# x11droid

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black)
![Podman](https://img.shields.io/badge/Podman-required-892CA0?logo=podman&logoColor=white)
![TUI](https://img.shields.io/badge/TUI-Bubble%20Tea-FF69B4?logo=charm&logoColor=white)

A CLI/TUI for running and managing [Waydroid](https://waydro.id) instances inside Podman containers on X11.

Each instance is an isolated Podman container with a nested Wayland compositor (cage/weston) that forwards its display back to your X11 session. The host stays clean — only two kernel modules need to be loaded.

## How it works

```
Your X11 desktop
  └─ Podman container (x11droid:latest)
       └─ cage (nested Wayland compositor) → forwards to $DISPLAY
            └─ waydroid session
                 └─ Android (LXC container, shared kernel)
```

The containers share your host kernel via `binder_linux` (Android IPC). No VM, no emulation — Android runs natively on your CPU.

## Prerequisites

- Linux with `binder_linux` kernel module available
- [Podman](https://podman.io)
- [just](https://just.systems)
- [asdf](https://asdf-vm.com) with the `golang` plugin (for building from source)
- X11 session (`echo $XDG_SESSION_TYPE` should print `x11`)

## Quick start

```bash
# 1. load kernel modules
just modules-load

# 2. build the container image (~500MB, takes a few minutes)
just image-build

# 3. run the TUI
just run
```

Or do steps 1+2 in one go:

```bash
just setup
just run
```

## TUI

```
x11droid  /  Dashboard
┌─────────────────────────────────────────────────────────┐
│ Instances (2)                                           │
│                                                         │
│ ▶ pixel9          a1b2c3d4e5f6  Up 2 hours    x11droid  │
│   pixel9-gapps    b2c3d4e5f6a1  Exited (0)   x11droid  │
│                                                         │
└─────────────────────────────────────────────────────────┘
↑↓ navigate  enter select  n new  s setup  r refresh  q quit
```

**Views:**

| View | How to open | Description |
|------|-------------|-------------|
| Dashboard | default | All instances with status |
| Instance | `enter` | Start / Stop / Remove / Shell / Logs |
| New Instance | `n` | Name input + GApps toggle |
| Setup | `s` | Kernel module status, image build |

**Key bindings:**

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate |
| `enter` | Select / confirm |
| `esc` | Back |
| `n` | New instance |
| `s` | Setup screen |
| `r` | Refresh |
| `space` | Toggle (spawn form) |
| `tab` | Next field (spawn form) |
| `q` / `ctrl+c` | Quit |

## Just recipes

```
just build          build the binary
just run            build and run the TUI
just install        install to $GOPATH/bin
just setup          load modules + build image
just modules-load   sudo modprobe binder_linux (+ ashmem_linux)
just modules-unload sudo rmmod
just modules-status check loaded modules
just image-build    podman build -t x11droid:latest
just image-clean    remove the container image
just tidy           go mod tidy
just vet            go vet ./...
just clean          remove built binary
```

## ARM translation (optional)

Most Play Store apps are pure Java and run without any translation layer. For apps with ARM-only native libraries, install `libndk` or `libhoudini` via [waydroid_script](https://github.com/casualsnek/waydroid_script) inside a running instance:

```bash
# open a shell into a running instance from the TUI (Instance → Shell)
# or directly:
podman exec -it <name> bash

# then inside the container:
git clone https://github.com/casualsnek/waydroid_script
cd waydroid_script
python3 -m venv venv && venv/bin/pip install -r requirements.txt
sudo venv/bin/python3 main.py install libndk
```

## Cleanup

```bash
# stop and remove all x11droid containers
podman ps -a --filter label=x11droid=true --format '{{.Names}}' | xargs -r podman rm -f

# unload kernel modules
just modules-unload

# remove the image
just image-clean
```
