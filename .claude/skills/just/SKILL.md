---
name: just
description: Use for ANY build, test, lint, vet, run, install, image, or debug operation in the x11droid repo. Always go through `just` recipes instead of raw `go`/`podman`/shell snippets; if the operation has no recipe yet, add one to the justfile and run that.
---

# Use `just` for operations

This repo is driven by [`just`](https://just.systems). **Do not run ad-hoc `go`/`podman`/shell snippets for operations that a recipe covers (or should cover).** Going through `just` keeps the pinned toolchain (`asdf exec go` / `asdf exec golangci-lint`), the version-stamping `-ldflags`, and the rootful-podman conventions consistent.

## Rules

1. **Prefer a recipe.** For build/test/lint/vet/run/install/image/debug, use the matching `just` recipe — never the underlying raw command.
2. **Missing recipe? Add it, then use it.** If you need an operation that has no recipe, *add a recipe to the `justfile`* (with a one-line `#` comment above it, matching the existing style and the `{{go}}`/`{{lint}}`/`{{image}}` variables), then run it via `just`. Don't run the one-off command directly. Also add it to the `default:` help block under the right section (BUILD / QUALITY / IMAGE / DEBUG).
3. **Validate before pushing.** Run `just check` (vet + test + lint) and `just build`; both must pass. Never push red. (See also CLAUDE.md.)
4. **Raw commands are the exception.** Only fall back to raw `go`/`podman` when something genuinely doesn't fit a recipe and isn't worth one (e.g. a quick `go test -run X ./pkg` while iterating). When in doubt, add the recipe.
5. **Not everything belongs in `just`.** Kernel-module load/unload and image build are managed in-app (`x11droid setup load|unload|build`) or the TUI Setup screen — not as `just` recipes.

## Current recipes (reference)

Run `just` (or `just --list`) for the live list. As of now:

| Group | Recipe | Does |
|-------|--------|------|
| BUILD | `just build` | compile the binary (version-stamped via `-ldflags`) |
| BUILD | `just run` | build + launch the TUI |
| BUILD | `just install` | install binary to `/usr/local/bin` (sudo) |
| BUILD | `just clean` | remove the built binary |
| BUILD | `just tidy` | `go mod tidy` |
| QUALITY | `just test` / `just test-v` | `go test ./...` (verbose variant) |
| QUALITY | `just lint` | `golangci-lint run ./...` |
| QUALITY | `just vet` | `go vet ./...` |
| QUALITY | `just check` | vet + test + lint (run before every push) |
| IMAGE | `just image-build` | `podman build -t x11droid:latest .` |
| IMAGE | `just image-clean` | remove the container image |
| DEBUG | `just adb <name>` | interactive Android root shell (`waydroid shell`) |
| DEBUG | `just apk <name> <path>` | install a local `.apk` into an instance |
| DEBUG | `just logcat <name>` | capture ~25s of Android logcat to `/tmp/lc.txt` |

The DEBUG recipes are also first-class CLI commands (`x11droid adb`/`install`/`logcat`).
