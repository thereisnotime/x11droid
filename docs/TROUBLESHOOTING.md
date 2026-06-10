# Troubleshooting

Most x11droid problems are environment-specific (GPU, kernel config, display
manager). This lists the ones seen in practice, with the underlying cause and
the fix. The app already handles most of these automatically — this is for when
something still goes wrong.

Two tools you'll want:

- **Container logs** — TUI: Instance → **Logs** (live), or `sudo x11droid logs <name>`. Shows the entrypoint + waydroid output.
- **Android logcat** — `just logcat <name>` (writes ~25s to `/tmp/lc.txt`). Use this for anything happening *inside* Android (crashes, reboots).

---

## "x11droid must run as root"

**Cause:** waydroid needs rootful podman to loop-mount the Android system image; rootless can't, even `--privileged`.

**Fix:** run `sudo x11droid`. Install to a system path so sudo finds it: `just install` (puts it in `/usr/local/bin`). If `sudo x11droid` says "command not found", run `hash -r`.

---

## weston window is black / "failed to create compositor backend" / "Invalid MIT-MAGIC-COOKIE-1 key"

**Cause:** the container (real root, different hostname) couldn't authenticate to your X server. `~/.Xauthority` is often stale; the live cookie lives elsewhere and rotates.

**Fix:** handled automatically — x11droid finds your live cookie via `/proc` and rewrites it hostname-agnostic (FamilyWild). If it still fails, as your user (not sudo) run `xhost +SI:localuser:root` and respawn.

---

## "Failed to create allocator" / cage exits immediately (NVIDIA)

**Cause:** cage's wlroots X11 backend can't allocate buffers on NVIDIA (no GPU EGL in the container, and NVIDIA's MIT-SHM isn't enough).

**Fix:** automatic — the entrypoint probes cage and falls back to **weston** (`--use-pixman`). You'll see `cage unsupported here — using weston` in the logs. Nothing to do.

---

## "Can't open /dev/anbox-binder" / no binder

**Cause:** kernels with `CONFIG_ANDROID_BINDER_DEVICES=""` create no binder device nodes. waydroid expects `anbox-binder`/`anbox-hwbinder`/`anbox-vndbinder`.

**Fix:** automatic — the entrypoint mounts **binderfs** and allocates the exact device names from `waydroid.cfg`. Requires `binder_linux` loaded on the host (Setup → Load Modules). Check with `grep binder /proc/filesystems`.

---

## "DBus.Error.NoReply" / "connection refused" during session start

**Cause:** no systemd in the container, so the system and session buses aren't running, or the container raced ahead of them.

**Fix:** automatic — the entrypoint starts its own system + session D-Bus and waits for each before use.

---

## GApps boots, then reboots in a loop right after the home screen

**Cause (most common):** `java.lang.OutOfMemoryError: pthread_create … Try again` in `system_server`. That's `EAGAIN` — hitting podman's default **2048 PID limit**. Android + GMS spawns more threads than that, system_server dies, Android reboots. (Confirm with `just logcat <name>`.)

**Fix:** automatic — spawns use `--pids-limit=-1`. Make sure you rebuilt the binary (`just install`) so the new spawn flag is in effect, then **remove and respawn** (don't reuse a container created before the fix).

**Secondary cause:** GApps ships ARM-native Google services that crash on x86 without translation. Enable the **ARM** toggle (installs libndk on first boot).

---

## Vanilla works but GApps doesn't

Almost always the two above: PID limit and/or missing ARM translation. Spawn GApps with **both** the GApps and ARM toggles on.

---

## libndk install failed

Read `~/.config/x11droid/instances/<name>/x11droid-libndk.log`. It runs `waydroid_script` and needs network (the container has it via NAT) plus `USER`/`HOME` (the entrypoint sets them). The install is one-time; the marker `.x11droid-libndk` in the data dir means it succeeded.

---

## Magisk (root) install failed

Same as libndk above, but read `~/.config/x11droid/instances/<name>/x11droid-magisk.log`. One-time; the marker `.x11droid-magisk` in the data dir means it succeeded. Open the Magisk app to grant `su`.

---

## Container stuck in "Stopping", can't Start

**Cause:** waydroid's loop mounts + Android LXC don't always tear down cleanly, so podman can't finish stopping.

**Fix:** use **Remove** (the app force-removes with `-t 0`), then spawn fresh. For waydroid, prefer **remove + spawn** over stop/start — restarting a container that loop-mounted an Android image is flaky. Manual: `sudo podman rm -f -t 0 <name>`.

---

## Play Store says "device not certified"

Not a crash — it's Google device registration. Sign in, then register the device's Google Services Framework ID at <https://www.google.com/android/uncertified>. Separate from anything x11droid does.

---

## CPU-Z shows my real (x86) CPU, not ARM

Expected. waydroid runs Android **natively on your host CPU** — no emulation. ARM translation (libndk) only patches individual apps' ARM libraries; it doesn't change the CPU. Device-name/model spoofing changes `Build.*` values apps read, but never the CPU (`/proc/cpuinfo`, CPU-Z SOC tab).

---

## Nothing renders and you're over SSH

x11droid forwards to a **local** X11 display over `/tmp/.X11-unix`. Run it at the machine (or an X session that owns `:0`); a forwarded SSH `$DISPLAY` won't work.
