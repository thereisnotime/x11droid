# Advanced usage

x11droid runs as **root** (`sudo x11droid`). Everything below assumes that.

## CLI reference

The bare command opens the TUI. Subcommands are scriptable:

| Command | Description |
|---------|-------------|
| `list` (`ls`, `ps`) | List all instances with status |
| `spawn <name> [flags]` | Create and start a new instance |
| `attach [name]` | List running instances, or (re)open the GUI for one |
| `hide <name>` | Close the GUI window but keep the instance running (Show UI / `attach` reopens it) |
| `start <name>` | Start a stopped instance |
| `stop <name>` | Stop a running instance (5s grace, then force) |
| `rm <name> [--purge]` (`remove`, `delete`) | Remove an instance; `--purge` also deletes its Android data |
| `prune [--all]` | Show per-instance disk usage and delete orphan data (no container); `--all` removes everything |
| `logs <name>` | Show the container/entrypoint logs |
| `shell <name>` | Open a bash shell inside the container |
| `config [--width --height --orientation --compositor]` | Show or set instance defaults |
| `setup [status\|load\|unload\|build]` | Module status, (un)load `binder_linux`, build the image |
| `version` | Version, commit, build date |

### `spawn` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--device-name <name>` | instance name | Android device/model name |
| `--gapps` | off | Google Play Store (LineageOS GApps image) |
| `--hidearm` | off | Install libndk ARM translation on first boot |
| `--fdroid` | off | Install F-Droid after first boot |
| `--aurora` | off | Install Aurora Store after first boot |
| `--obtainium` | off | Install Obtainium after first boot |
| `--shelter` | off | Install Shelter after first boot |
| `--dev-options` | off | Enable Android Developer Options on first boot |
| `--root` | off | Install Magisk (root) on first boot |
| `--no-pv` | off | Disable the persistent volume (data won't survive removal) |

Resolution / orientation / compositor come from the saved **Config** (TUI: `c`), persisted to `~/.config/x11droid/config.json`.

Example:

```bash
sudo x11droid spawn pixel9 --device-name "Pixel 9 Pro" --gapps --hidearm --fdroid --aurora --obtainium --shelter
sudo x11droid logs pixel9          # watch first boot (downloads ~1GB, installs libndk + apps)
sudo x11droid attach pixel9        # reopen the window if you closed it
```

## How it works (internals)

Each instance is a rootful, privileged Podman container that runs a real
Android LXC container via waydroid, displayed through a nested weston
compositor forwarded to the host X11 server.

- **Rootful podman** — waydroid loop-mounts `system.img`/`vendor.img`; rootless can't associate a loop device (`losetup: Permission denied`) even `--privileged`. So all podman calls run as root, and x11droid resolves the *invoking* user's display/auth/home from `SUDO_USER`.
- **Compositor** — cage's wlroots X11 backend can't allocate on NVIDIA, so the entrypoint probes cage and falls back to **weston** with the pixman renderer and **kiosk-shell** (fullscreen, no panel). Output goes to the host `:0` over the mounted `/tmp/.X11-unix` socket — not the network.
- **X11 auth** — the container connects as real root with a different hostname, so x11droid finds the live `XAUTHORITY` via `/proc` and writes a hostname-agnostic (FamilyWild) cookie.
- **binder** — kernels with `CONFIG_ANDROID_BINDER_DEVICES=""` create no device nodes, so the entrypoint mounts **binderfs** and allocates the device names from `waydroid.cfg` (`anbox-binder`, …) via the `BINDER_CTL_ADD` ioctl.
- **D-Bus** — no systemd, so the entrypoint runs its own **system** and **session** buses and waits for each before use.
- **Networking** — *not* `--network=host` (rootless can't modify the host netns); the container gets its own netns so waydroid can create the `waydroid0` bridge.
- **cgroups / threads** — `--cgroupns=host` (controller delegation) and `--pids-limit=-1` (Android/GMS exceeds podman's 2048 default → `pthread_create` reboot loop).

The container entrypoint is `internal/container/entrypoint.sh` (embedded in the binary and copied into the image).

## Apps & ARM translation

- The **F-Droid**, **Aurora**, and **Shelter** toggles each install that app from the F-Droid repo (current version resolved via its API); **Obtainium** installs the native x86_64 build from GitHub releases. Only the toggles you enable are installed, once Android finishes booting. One-time, marked done in the data dir.
- The **ARM** toggle installs **libndk** via [waydroid_script](https://github.com/casualsnek/waydroid_script) on first boot. Needed for apps (and GApps services) that ship ARM-only native libraries — it translates ARM→x86 per-app. The base system stays native x86 (fast).

## Root (Magisk)

The **Root** toggle (or `--root`) installs **Magisk Delta** via [waydroid_script](https://github.com/casualsnek/waydroid_script) on first boot (one-time, marked done in the data dir; install failures are logged to `instances/<name>/x11droid-magisk.log`). The `su` daemon (magiskd) comes up at boot regardless — verify with `just adb <name>` then `su -c id`.

waydroid_script only pre-seeds the manager apk into the overlay, so the daemon logs `pkg: cannot find io.github.huskydg.magisk` and the **Magisk app won't open / show as rooted**. x11droid works around this by installing the bundled apk properly via PackageManager once Android finishes booting (logged to `instances/<name>/x11droid-magisk-app.log`, marked with `.x11droid-magisk-app`). If the app still misbehaves on an instance created before this fix, respawn it or reinstall the apk: `just apk <name> <path-to-magisk.apk>`.

Root won't pass Play Integrity / SafetyNet — Magisk is detectable here and there's no hardware-backed attestation in waydroid.

## Developer Options

The **Dev Options** toggle (or `--dev-options`) flips `development_settings_enabled` and `adb_enabled` once Android finishes booting, so the Developer Options menu and adb are available without tapping the build number seven times.

## Debug & dev helpers

These `just` recipes act on a running instance (they `sudo podman exec` into it):

| Recipe | Does |
|--------|------|
| `just adb <name>` | Interactive Android **root shell** (`waydroid shell`) — e.g. `magisk -v`, `su -c id` |
| `just apk <name> <path>` | Install a local `.apk` into the instance (`waydroid app install`) — the `adb install` equivalent |
| `just logcat <name>` | Capture ~25s of Android **logcat** to `/tmp/lc.txt` |

```bash
just adb pixel9
just apk pixel9 ~/Downloads/app.apk
just logcat pixel9
```

`just adb` opens a shell, not the full `adb` binary. For the real `adb` toolchain (push/pull, `adb logcat -f`, Android Studio), enable adb-over-TCP inside Android and `adb connect` to the instance's `waydroid0` bridge IP.

## Device naming / spoofing

The **Device** field (or `--device-name`) sets `ro.product.model` (via `waydroid_base.prop`) and the Settings device name. This changes what apps read (`Build.MODEL`, About phone) — useful for naming and some app-compat checks.

It does **not** change the CPU: `/proc/cpuinfo` and CPU-Z's SOC tab always show your real x86 host CPU. Spoofing won't pass hardware attestation / Play Integrity.

## Persistence, data, cleanup

- Each instance keeps its data in `~/.config/x11droid/instances/<name>/` — the base images, overlays, config, and Android `/data` (apps/accounts/settings, persisted into `data/`). Spawning the same name reuses it (no re-download); installed apps survive restarts.
- `rm` removes the container but **keeps** the data dir. A data dir whose container is gone is an **orphan** — invisible in the dashboard but still using disk.
- The container image, Containerfile, entrypoint and a fake `modprobe` live under `~/.config/x11droid/`.

**Cleanup** (each instance's base images are ~3 GB, so this adds up):

```bash
# show per-instance disk usage and delete orphan data (safe — orphans have no container)
sudo x11droid prune
# wipe EVERYTHING (all containers + all data)
sudo x11droid prune --all
# delete one instance's data too
sudo x11droid rm <name> --purge        # or TUI: Instance → Purge (asks to confirm)
# remove the image
sudo podman rmi -f x11droid:latest
```

In the TUI: **Setup → Prune Orphan Data**, or **Instance → Purge** for a specific one.

## Multiple instances

Spawn as many as you like with different names; each is an isolated container
with its own data dir and Android device. They share the host kernel's
`binder_linux` and your X server. Mind the resources — each Android boot is a
full system.
