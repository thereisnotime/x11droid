# Contributing

Thanks for helping improve x11droid. It's under active development, so issues and PRs are very welcome.

## Before you start

- Read the [README](README.md) and [docs/ADVANCED.md](docs/ADVANCED.md).
- x11droid only runs **rootful** (`sudo x11droid`) on a local **X11** session — that's the supported runtime. Display/GPU/kernel specifics matter a lot here, so keep them in mind when testing.

## Dev setup

Go is pinned via [asdf](https://asdf-vm.com) (`.tool-versions`) and everything is driven through [just](https://just.systems):

```bash
just build      # build the binary
just run        # build + run the TUI
just check      # vet + test + lint  (run before every PR — CI runs the same)
just tidy       # go mod tidy
```

Only fall back to raw `go`/`podman` when no `just` recipe covers what you need.

## Pull requests

- Keep changes focused and match the surrounding code style.
- `just check` must pass (CI enforces vet + test + golangci-lint).
- Describe **what** you changed and **how you tested it** — include your host GPU, kernel, and whether the instance used GApps/ARM/Root, since most bugs are environment-specific.
- Commit messages: natural, human tone. No `Co-Authored-By` trailers.

## Reporting bugs

Open a [bug report](https://github.com/thereisnotime/x11droid/issues/new?template=bug_report.yml) — the form collects the version, host environment, and logs needed to debug.

## License

By contributing, you agree your contributions are licensed under the project's [GPL-3.0](LICENSE) license.
