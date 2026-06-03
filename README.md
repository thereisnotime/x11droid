# x11droid

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black)
![Podman](https://img.shields.io/badge/Podman-required-892CA0?logo=podman&logoColor=white)
![TUI](https://img.shields.io/badge/TUI-Bubble%20Tea-FF69B4?logo=charm&logoColor=white)

A CLI/TUI for running and managing [Waydroid](https://waydro.id) instances inside Podman containers on X11.

Each instance is an isolated Podman container with a nested Wayland compositor (cage/weston) that forwards its display back to your X11 session. The host stays clean — only two kernel modules need to be loaded.

## How it works

```mermaid
graph TD
    subgraph HOST["Host — X11 Session"]
        TUI["x11droid TUI\nGo · Bubble Tea"]
        PODMAN["Podman"]

        subgraph CONTAINER["Podman Container  ─  x11droid:latest"]
            CAGE["cage / weston\nnested Wayland compositor"]

            subgraph WD["Waydroid Session"]
                WDSVC["waydroid services\nAndroid HAL / SurfaceFlinger"]

                subgraph LXC["Android  ─  LXC Container"]
                    INIT["Android init + system services"]
                    APPS["Android Apps  ·  ART runtime"]
                end
            end
        end

        subgraph KERNEL["Linux Kernel  (shared)"]
            BINDER["binder_linux\nAndroid IPC"]
            ASHMEM["ashmem_linux\nshared memory"]
        end
    end

    X11["X11 Display Server\n:0  /tmp/.X11-unix"]

    TUI -- "podman CLI\nspawn · start · stop · remove" --> PODMAN
    PODMAN -- "create / manage" --> CONTAINER
    CAGE -- "X11 client\n DISPLAY env + socket" --> X11
    CAGE -- "Wayland socket\nwayland-0" --> WDSVC
    WDSVC --> LXC
    INIT --> APPS
    LXC -- "binder IPC" --> BINDER
    LXC -- "shared mem" --> ASHMEM
    BINDER --> KERNEL
    ASHMEM --> KERNEL

    style HOST fill:#1a1a2e,stroke:#444,color:#ccc
    style CONTAINER fill:#16213e,stroke:#555,color:#ccc
    style WD fill:#0f3460,stroke:#666,color:#ccc
    style LXC fill:#533483,stroke:#777,color:#ddd
    style KERNEL fill:#1a1a1a,stroke:#333,color:#aaa
    style TUI fill:#2d6a4f,stroke:#52b788,color:#fff
    style PODMAN fill:#892CA0,stroke:#c77dff,color:#fff
    style CAGE fill:#1d3557,stroke:#457b9d,color:#fff
    style X11 fill:#333,stroke:#666,color:#bbb
    style WDSVC fill:#023e8a,stroke:#0096c7,color:#fff
    style BINDER fill:#3d0000,stroke:#9d0208,color:#ffc,font-size:12px
    style ASHMEM fill:#3d0000,stroke:#9d0208,color:#ffc,font-size:12px
    style INIT fill:#4a1942,stroke:#9b5de5,color:#fff
    style APPS fill:#4a1942,stroke:#9b5de5,color:#fff
```

**Control plane:** x11droid manages container lifecycle via the `podman` CLI.

**Display path:** cage runs as an X11 client inside the container, forwarding its Wayland compositor output to your `$DISPLAY` socket. Android surfaces appear as regular X11 windows.

**Kernel path:** Android IPC (`binder`) runs directly on your host kernel — no emulation, no VM. Apps execute natively at full CPU speed.

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
