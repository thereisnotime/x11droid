# Add custom image source to spawn

## Why

Official Waydroid images top out at Android 13 (LineageOS 20). Users who want a newer build — e.g. a community Android 15 (LineageOS 22) `waydroid_x86_64` image — currently can't, because `spawn` always uses `waydroid init`'s official download. There is no supported way to point an instance at a custom system/vendor image.

## What Changes

- `spawn` gains `--system-image` and `--vendor-image` (a local path or `http(s)` URL to a raw `.img` or a `.zip`); both are required together.
- A new `internal/image` package resolves each source: download (when a URL), extract the matching `.img` from a `.zip`, and validate it (reject truncated downloads and still-sparse images).
- Resolved images are written to `~/.config/x11droid/extra-images/<name>/` and mounted at `/etc/waydroid-extra/images`, which `waydroid init` uses instead of downloading the official image.
