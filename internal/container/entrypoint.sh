#!/bin/bash
# x11droid container entrypoint. Brings up a system D-Bus, the waydroid
# container manager, a nested compositor on the host X11 display, then the
# Android UI. Copied to /usr/bin/waydroid-session.sh in the image.
set -u

COMPOSITOR="${WAYDROID_COMPOSITOR:-auto}"
GAPPS="${WAYDROID_GAPPS:-}"
HIDEARM="${WAYDROID_HIDEARM:-}"
W="${WAYDROID_WIDTH:-540}"
H="${WAYDROID_HEIGHT:-960}"
X11DROID_NAME="${X11DROID_NAME:-instance}"

cleanup() {
  trap - EXIT INT TERM HUP
  waydroid session stop 2>/dev/null || true
  waydroid container stop 2>/dev/null || true
  killall waydroid cage weston dbus-daemon 2>/dev/null || true
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

# --- compositor + UI ------------------------------------------------------
run_weston() {
  # kiosk-shell = single fullscreen app, no panel/taskbar/background — the
  # Android window fills the output (what cage would do if it worked on NVIDIA).
  weston --backend=x11-backend.so --use-pixman --shell=kiosk-shell.so --width="$W" --height="$H" >/tmp/weston.log 2>&1 &
  for _ in $(seq 1 30); do
    ls "$XDG_RUNTIME_DIR"/wayland-[0-9]* >/dev/null 2>&1 && break
    sleep 0.5
  done
  WAYLAND_DISPLAY="$(basename "$(ls "$XDG_RUNTIME_DIR"/wayland-[0-9]* 2>/dev/null | head -1)")"
  export WAYLAND_DISPLAY
  echo "[x11droid] weston up on ${WAYLAND_DISPLAY:-?} (${W}x${H})"
  title_window /tmp/weston.log
  show_ui
}

run_cage() {
  cage -s -- bash -lc show_ui
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
