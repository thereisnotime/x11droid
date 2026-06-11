#!/bin/bash
# x11droid container entrypoint. Brings up a system D-Bus, the waydroid
# container manager, a nested compositor on the host X11 display, then the
# Android UI. Copied to /usr/bin/waydroid-session.sh in the image.
set -u

COMPOSITOR="${WAYDROID_COMPOSITOR:-auto}"
GAPPS="${WAYDROID_GAPPS:-}"
HIDEARM="${WAYDROID_HIDEARM:-}"
ROOT="${WAYDROID_ROOT:-}"
DEVOPTS="${WAYDROID_DEVOPTS:-}"
APPS="${WAYDROID_APPS:-}"
W="${WAYDROID_WIDTH:-540}"
H="${WAYDROID_HEIGHT:-960}"
X11DROID_NAME="${X11DROID_NAME:-instance}"
# Android device/model name — explicit override, else the instance name.
DEVICE_NAME="${WAYDROID_DEVICE:-$X11DROID_NAME}"
# Per-instance wayland socket name so multiple instances sharing the host
# XDG_RUNTIME_DIR don't collide (a shared "wayland-1" let a second instance
# hijack the first's compositor).
WL_SOCKET="wl-${X11DROID_NAME}"

cleanup() {
  trap - EXIT INT TERM HUP
  waydroid session stop 2>/dev/null || true
  waydroid container stop 2>/dev/null || true
  # pkill (procps) — psmisc/killall isn't installed in the image.
  for p in waydroid cage weston dbus-daemon; do
    pkill -x "$p" 2>/dev/null || killall "$p" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM HUP

bus_ready() {
  dbus-send --system --print-reply --dest=org.freedesktop.DBus \
    / org.freedesktop.DBus.ListNames >/dev/null 2>&1
}

manager_ready() {
  dbus-send --system --print-reply --dest=org.freedesktop.DBus \
    / org.freedesktop.DBus.GetNameOwner string:id.waydro.Container >/dev/null 2>&1
}

# --- system bus -----------------------------------------------------------
# No systemd in here, so run our own. Clear any stale socket first, then wait
# until the bus actually answers before anything tries to use it (a race here
# is what makes the container manager die with "connection refused").
rm -f /run/dbus/system_bus_socket /run/dbus/pid 2>/dev/null || true
mkdir -p /run/dbus
dbus-daemon --system --fork 2>/dev/null || true
for _ in $(seq 1 40); do
  bus_ready && break
  sleep 0.25
done
if bus_ready; then
  echo "[x11droid] system bus ready"
else
  echo "[x11droid] WARNING: system bus did not come up" >&2
fi

# Session bus — waydroid's session_manager registers id.waydro.Session here.
# The host's $XDG_RUNTIME_DIR/bus is mounted in but doesn't answer a
# root-in-container client (NoReply), so run our own and point clients at it.
export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/dbus/session_bus_socket"
rm -f /run/dbus/session_bus_socket 2>/dev/null || true
dbus-daemon --session --address="$DBUS_SESSION_BUS_ADDRESS" --fork 2>/dev/null || true
echo "[x11droid] session bus: $DBUS_SESSION_BUS_ADDRESS"

# Persist Android /data. waydroid stores userdata at ~/.local/share/waydroid/data
# — inside the container's ephemeral filesystem — so without this, installed
# apps/accounts/settings vanish on every restart. Redirect it into the mounted
# /var/lib/waydroid volume.
mkdir -p /var/lib/waydroid/data /root/.local/share/waydroid
[ -e /root/.local/share/waydroid/data ] || ln -s /var/lib/waydroid/data /root/.local/share/waydroid/data

# --- first-run init (downloads ~1GB) --------------------------------------
if [ ! -f /var/lib/waydroid/images/system.img ]; then
  echo "[x11droid] First run: initialising Waydroid (downloads ~1GB, please wait)..."
  if [ -n "$GAPPS" ]; then
    waydroid init -f -s GAPPS
  else
    waydroid init -f
  fi
  if [ $? -ne 0 ]; then
    echo "[x11droid] waydroid init failed — check logs with: x11droid logs <name>"
    exit 1
  fi
  echo "[x11droid] Init done."
fi

# ARM translation (libndk) via waydroid_script. GApps x86_64 images ship
# ARM-native Google services that crash without it (boot-loop after the home
# screen). One-time per instance; marked done so reboots skip it.
if [ -n "$HIDEARM" ] && [ ! -f /var/lib/waydroid/.x11droid-libndk ]; then
  echo "[x11droid] installing ARM translation (libndk) — a few minutes..."
  (
    # Subshell so USER/HOME (unset for bare root, but waydroid_script needs
    # them) don't leak into the rest of the entrypoint.
    export USER=root HOME=/root
    d=/opt/waydroid_script
    [ -d "$d/.git" ] || git clone --depth 1 https://github.com/casualsnek/waydroid_script "$d"
    cd "$d" && \
    { [ -d venv ] || python3 -m venv venv; } && \
    venv/bin/pip install -q -r requirements.txt && \
    venv/bin/python3 main.py install libndk
  ) >/var/lib/waydroid/x11droid-libndk.log 2>&1 \
    && { touch /var/lib/waydroid/.x11droid-libndk; echo "[x11droid] libndk installed"; } \
    || echo "[x11droid] libndk install failed — see instances/<name>/x11droid-libndk.log" >&2
  cd / || true
fi

# Magisk (root) via waydroid_script. One-time per instance; marked done so
# reboots skip it. Open the Magisk app afterwards to grant su.
if [ -n "$ROOT" ] && [ ! -f /var/lib/waydroid/.x11droid-magisk ]; then
  echo "[x11droid] installing Magisk (root) — a few minutes..."
  (
    export USER=root HOME=/root
    d=/opt/waydroid_script
    [ -d "$d/.git" ] || git clone --depth 1 https://github.com/casualsnek/waydroid_script "$d"
    cd "$d" && \
    { [ -d venv ] || python3 -m venv venv; } && \
    venv/bin/pip install -q -r requirements.txt && \
    venv/bin/python3 main.py install magisk
  ) >/var/lib/waydroid/x11droid-magisk.log 2>&1 \
    && { touch /var/lib/waydroid/.x11droid-magisk; echo "[x11droid] Magisk installed"; } \
    || echo "[x11droid] Magisk install failed — see instances/<name>/x11droid-magisk.log" >&2
  cd / || true
fi

# Register the Magisk manager app with PackageManager. waydroid_script only
# pre-seeds the apk into the overlay, so the magisk daemon boots but logs
# "pkg: cannot find io.github.huskydg.magisk" and the app won't open / show as
# rooted. Installing the bundled apk properly once Android is up fixes that.
if [ -n "$ROOT" ] && [ ! -f /var/lib/waydroid/.x11droid-magisk-app ]; then
  (
    for _ in $(seq 1 120); do
      [ "$(waydroid prop get sys.boot_completed 2>/dev/null | tr -d '\r ')" = "1" ] && break
      sleep 5
    done
    apk=/var/lib/waydroid/overlay/system/etc/init/magisk/magisk.apk
    [ -f "$apk" ] && waydroid app install "$apk" \
      && touch /var/lib/waydroid/.x11droid-magisk-app \
      && echo "[x11droid] Magisk manager app registered"
  ) >/var/lib/waydroid/x11droid-magisk-app.log 2>&1 &
fi

# wait_for_boot blocks until Android reports boot complete, then waits a little
# longer for the package/settings services to settle — running `settings put`
# the instant sys.boot_completed flips races them and silently no-ops.
wait_for_boot() {
  local _
  for _ in $(seq 1 120); do
    [ "$(waydroid prop get sys.boot_completed 2>/dev/null | tr -d '\r ')" = "1" ] && { sleep 8; return 0; }
    sleep 5
  done
  echo "[x11droid] WARNING: boot did not complete in time" >&2
  return 1
}

# put_global sets an Android global setting, retrying until `settings get`
# confirms it stuck (the settings service rejects writes for a few seconds
# after boot, which is why the old fire-and-forget put silently failed).
put_global() {
  local k="$1" v="$2" i got
  for i in $(seq 1 12); do
    # Write as the shell user (uid 2000 = com.android.shell), not root: guarded
    # settings like development_settings_enabled run a checkPackage on the caller,
    # and root (uid 0) has no package -> getCallingPackage() NPEs. uid 2000 does.
    # Use su's positional command form (no `-c`): `waydroid shell` is argparse-based
    # and steals a `-c` flag ("unrecognized arguments: -c ..."), so it never reaches
    # Android. Fall back to a plain root put for unguarded keys / images without su.
    waydroid shell su shell settings put global "$k" "$v" >/dev/null 2>&1 \
      || waydroid shell settings put global "$k" "$v" >/dev/null 2>&1
    got="$(waydroid shell settings get global "$k" 2>/dev/null | tr -d '\r ')"
    if [ "$got" = "$v" ]; then
      echo "[x11droid] set global $k=$v"
      return 0
    fi
    sleep 5
  done
  echo "[x11droid] WARNING: could not set global $k=$v (last read: '$got')" >&2
  return 1
}

# Device name — set the Android model (CPU-Z / About phone) via
# waydroid_base.prop, and the Settings "Device name" once booted.
if [ -n "$DEVICE_NAME" ] && [ "$DEVICE_NAME" != "instance" ] && [ -f /var/lib/waydroid/waydroid_base.prop ]; then
  sed -i '/^ro\.product\.model=/d' /var/lib/waydroid/waydroid_base.prop
  echo "ro.product.model=$DEVICE_NAME" >> /var/lib/waydroid/waydroid_base.prop
  ( wait_for_boot && put_global device_name "$DEVICE_NAME" ) \
    >>/var/lib/waydroid/x11droid-devopts.log 2>&1 &
fi

# Developer Options — enable the Android dev-settings menu and adb once booted.
if [ -n "$DEVOPTS" ]; then
  ( wait_for_boot \
      && put_global development_settings_enabled 1 \
      && put_global adb_enabled 1 ) \
    >>/var/lib/waydroid/x11droid-devopts.log 2>&1 &
fi

# --- binder via binderfs --------------------------------------------------
# Kernels with CONFIG_ANDROID_BINDER_DEVICES="" create no binder nodes. binderfs
# is available, so mount it and allocate the nodes. Crucially, use the device
# NAMES from waydroid.cfg (it usually picks "anbox-binder" etc.) — creating the
# wrong name (e.g. plain "binder") makes gbinder fail with
# "Can't open /dev/anbox-binder" and tricks waydroid into skipping its own setup.
cfg_get() { awk -F'=' -v k="$1" '$1 ~ "^[ \t]*"k"[ \t]*$"{gsub(/[ \t]/,"",$2);print $2;exit}' /var/lib/waydroid/waydroid.cfg 2>/dev/null; }
BINDER_NODE="$(cfg_get binder)";      BINDER_NODE="${BINDER_NODE:-binder}"
VND_NODE="$(cfg_get vndbinder)";      VND_NODE="${VND_NODE:-vndbinder}"
HW_NODE="$(cfg_get hwbinder)";        HW_NODE="${HW_NODE:-hwbinder}"
if [ ! -e "/dev/$BINDER_NODE" ]; then
  echo "[x11droid] provisioning binder nodes ($BINDER_NODE $VND_NODE $HW_NODE) via binderfs..."
  mkdir -p /dev/binderfs
  mountpoint -q /dev/binderfs || mount -t binder binder /dev/binderfs 2>/dev/null
  if [ -e /dev/binderfs/binder-control ]; then
    python3 - "$BINDER_NODE" "$VND_NODE" "$HW_NODE" <<'PY' || echo "[x11droid] binderfs alloc failed" >&2
import fcntl, struct, os, sys
BINDER_CTL_ADD = (3 << 30) | (264 << 16) | (98 << 8) | 1  # IOWR('b',1,sizeof)
fd = os.open("/dev/binderfs/binder-control", os.O_RDONLY)
for name in sys.argv[1:]:
    try:
        fcntl.ioctl(fd, BINDER_CTL_ADD, struct.pack("256sII", name.encode(), 0, 0))
    except FileExistsError:
        pass
os.close(fd)
PY
    for d in "$BINDER_NODE" "$VND_NODE" "$HW_NODE"; do
      [ -e "/dev/$d" ] || ln -s "/dev/binderfs/$d" "/dev/$d"
      chmod 0666 "/dev/binderfs/$d" 2>/dev/null || true
    done
  else
    echo "[x11droid] WARNING: binderfs has no binder-control — binder unavailable" >&2
  fi
fi
echo "[x11droid] binder node: $(ls -lL "/dev/$BINDER_NODE" 2>&1)"

# --- container manager (LXC + binder) -------------------------------------
echo "[x11droid] starting container manager..."
waydroid container start >/var/lib/waydroid/x11droid-cm.log 2>&1 &
for _ in $(seq 1 60); do
  manager_ready && break
  sleep 0.5
done
if manager_ready; then
  echo "[x11droid] container manager ready"
else
  echo "[x11droid] WARNING: container manager not responding — session may fail" >&2
fi

# show_ui starts the Android session + UI, retrying because the manager often
# claims its bus name a moment before it can service StartSession (instant
# "NoReply"). On repeated failure it dumps the real error for diagnosis.
show_ui() {
  local attempt
  for attempt in 1 2 3 4 5 6; do
    if waydroid show-full-ui; then
      return 0
    fi
    echo "[x11droid] session start failed (attempt ${attempt}/6) — retrying in 4s..." >&2
    waydroid session stop 2>/dev/null || true
    sleep 4
  done
  # Persist a full diagnostic bundle so it can be inspected from the host.
  {
    echo "===== x11droid debug $(date -u) ====="
    echo "--- env ---"; env | grep -iE "DISPLAY|WAYLAND|XDG|DBUS" | sort
    echo "--- binder nodes ---"; ls -lL /dev/binder /dev/hwbinder /dev/vndbinder 2>&1
    echo "--- binderfs mount ---"; mount | grep -i binder
    echo "--- waydroid -d status ---"; waydroid -d status 2>&1 | tail -20
    echo "--- container manager log ---"; tail -40 /var/lib/waydroid/x11droid-cm.log 2>&1
    echo "--- waydroid.log ---"; tail -50 /var/lib/waydroid/waydroid.log 2>&1
  } > /var/lib/waydroid/x11droid-debug.log 2>&1
  echo "[x11droid] session still failing — wrote /var/lib/waydroid/x11droid-debug.log" >&2
  return 1
}
export -f show_ui

# install_apps waits for Android to finish booting, then installs the apps named
# in the comma-separated $APPS list (fdroid/aurora/shelter from the F-Droid repo,
# resolving the current version via its API; obtainium from GitHub releases).
# Runs in the background; marks done so it only installs once.
fdroid_install() { # $1 = package id, $2 = friendly name
  local pkg="$1" name="$2" vc apk
  vc="$(curl -fsSL "https://f-droid.org/api/v1/packages/$pkg" 2>/dev/null | grep -oE '"suggestedVersionCode"[ :]+[0-9]+' | grep -oE '[0-9]+' | head -1)"
  [ -n "$vc" ] || { echo "[x11droid] $name: could not resolve version" >&2; return 1; }
  apk="/tmp/x11droid-apks/${pkg}.apk"
  curl -fsSL -o "$apk" "https://f-droid.org/repo/${pkg}_${vc}.apk" \
    || { echo "[x11droid] $name: download failed" >&2; return 1; }
  waydroid app install "$apk" && echo "[x11droid] $name installed" \
    && echo "$name" >> /var/lib/waydroid/.x11droid-apps
}
github_install() { # $1 = owner/repo, $2 = asset regex, $3 = friendly name
  local repo="$1" pat="$2" name="$3" url apk
  url="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" 2>/dev/null | grep -oE 'https://[^"]+\.apk' | grep -E "$pat" | head -1)"
  [ -n "$url" ] || { echo "[x11droid] $name: no matching release asset" >&2; return 1; }
  apk="/tmp/x11droid-apks/$(basename "$url")"
  curl -fsSL -o "$apk" "$url" \
    || { echo "[x11droid] $name: download failed" >&2; return 1; }
  waydroid app install "$apk" && echo "[x11droid] $name installed" \
    && echo "$name" >> /var/lib/waydroid/.x11droid-apps
}
want_app() { # $1 = app key — true if present in the comma list $APPS
  case ",${APPS:-}," in *",$1,"*) return 0 ;; *) return 1 ;; esac
}
install_apps() {
  for _ in $(seq 1 120); do
    [ "$(waydroid prop get sys.boot_completed 2>/dev/null | tr -d '\r ')" = "1" ] && break
    sleep 5
  done
  mkdir -p /tmp/x11droid-apks
  want_app fdroid    && fdroid_install org.fdroid.fdroid "F-Droid"
  want_app aurora    && fdroid_install com.aurora.store "Aurora Store"
  want_app shelter   && fdroid_install net.typeblog.shelter "Shelter"
  want_app obtainium && github_install ImranR98/Obtainium 'app-x86_64-release\.apk$' "Obtainium"
  touch /var/lib/waydroid/.x11droid-apps
}

# title_window renames weston's X11 output window to
# "x11droid - <name> - weston[ - Android <ver>]". The Android version isn't
# known until it boots, so a background task fills it in once available.
title_window() {
  local log="$1" wid base
  for _ in $(seq 1 20); do
    wid="$(grep -oE 'window id [0-9]+' "$log" 2>/dev/null | head -1 | awk '{print $NF}')"
    [ -n "$wid" ] && break
    sleep 0.3
  done
  [ -n "$wid" ] || return
  base="x11droid - ${X11DROID_NAME:-instance} - weston"
  xdotool set_window --name "$base" "$wid" 2>/dev/null || true
  (
    for _ in $(seq 1 120); do
      ver="$(waydroid prop get ro.build.version.release 2>/dev/null | tr -d '\r\n ')"
      if [ -n "$ver" ]; then
        xdotool set_window --name "$base - Android $ver" "$wid" 2>/dev/null || true
        break
      fi
      sleep 5
    done
  ) &
}

# Selected apps — once, in the background, after Android boots.
if [ -n "$APPS" ] && [ ! -f /var/lib/waydroid/.x11droid-apps ]; then
  echo "[x11droid] will install selected apps ($APPS) once Android finishes booting..."
  install_apps >/var/lib/waydroid/x11droid-apps.log 2>&1 &
fi

# --- compositor + UI ------------------------------------------------------
run_weston() {
  # kiosk-shell = single fullscreen app, no panel/taskbar/background — the
  # Android window fills the output (what cage would do if it worked on NVIDIA).
  # --socket gives this instance a unique wayland socket so it doesn't collide
  # with another instance in the shared XDG_RUNTIME_DIR.
  weston --backend=x11-backend.so --use-pixman --shell=kiosk-shell.so \
    --socket="$WL_SOCKET" --width="$W" --height="$H" >/tmp/weston.log 2>&1 &
  for _ in $(seq 1 30); do
    [ -S "$XDG_RUNTIME_DIR/$WL_SOCKET" ] && break
    sleep 0.5
  done
  export WAYLAND_DISPLAY="$WL_SOCKET"
  echo "[x11droid] weston up on ${WAYLAND_DISPLAY} (${W}x${H})"
  title_window /tmp/weston.log
  # Background the session so PID 1 doesn't depend on it — Hide UI / Show UI can
  # tear down and relaunch the compositor without exiting the entrypoint.
  show_ui &
}

run_cage() {
  cage -s -- bash -lc show_ui &
}

case "$COMPOSITOR" in
  cage)
    run_cage
    ;;
  weston)
    run_weston
    ;;
  auto)
    # cage/wlroots X11 backend cannot allocate buffers on NVIDIA; probe it
    # quickly and fall back to weston (pixman) which works everywhere.
    if cage -s -- true >/dev/null 2>&1; then
      echo "[x11droid] compositor: cage"
      run_cage
    else
      echo "[x11droid] cage unsupported here (GPU/allocator) — using weston"
      run_weston
    fi
    ;;
  *)
    echo "Unknown compositor: $COMPOSITOR" >&2
    exit 1
    ;;
esac

# Keep PID 1 (and thus the container) alive independent of the compositor and
# Android session, so the window can be closed / hidden / re-shown without
# tearing the instance down. cleanup() still runs on a real stop (SIGTERM/EXIT).
# `wait` on a backgrounded sleep keeps bash responsive to those signals.
while :; do
  sleep 2147483647 &
  wait $! || true
done
