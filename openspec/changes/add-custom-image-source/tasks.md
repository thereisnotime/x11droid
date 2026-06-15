# Tasks

## 1. Image resolution

- [x] `internal/image.Resolve(src, dst, imgName)` — path/URL, download, unzip, validate
- [x] Unit tests (local raw, local zip, URL raw/zip, missing/too-small/sparse/zip-missing/404)

## 2. Spawn integration

- [x] `SpawnOpts.SystemImage` / `VendorImage`
- [x] Resolve both into the extra-images dir and mount at `/etc/waydroid-extra/images`
- [x] Entrypoint skips the official download when `WAYDROID_CUSTOM_IMAGES` is set

## 3. CLI

- [x] `spawn --system-image` / `--vendor-image` (both required together)

## 4. Docs

- [x] ADVANCED.md spawn flags + custom-images section

## 5. Verification (on hardware)

- [ ] Confirm `waydroid init` picks up `/etc/waydroid-extra/images` and boots a custom Android 15 image
- [ ] Confirm libndk/Magisk paths behave (or document gaps) on LineageOS 22
