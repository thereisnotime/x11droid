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

## Specs (OpenSpec)

This repo uses [OpenSpec](https://github.com/Fission-AI/OpenSpec) for spec-driven development — specs live in `openspec/specs/`, change proposals in `openspec/changes/`. For a non-trivial feature or behavior change, **create a change proposal first** (`openspec` CLI, or the OpenSpec Claude skills / `/opsx:propose`), get it reviewed, implement against it, then archive (`openspec archive`). Run `openspec list` / `openspec view` to see current specs and changes. Small fixes and docs don't need a proposal.

## Pull requests

- Keep changes focused and match the surrounding code style.
- **Validate with `just` before pushing** — `just check` (vet + test + golangci-lint) and `just build` must pass. CI runs the same; never push red.
- Describe **what** you changed and **how you tested it** — include your host GPU, kernel, and whether the instance used GApps/ARM/Root, since most bugs are environment-specific.

## Commits & versioning

- **[Conventional Commits](https://www.conventionalcommits.org/) are mandatory.** Format: `type(scope): summary` — types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`. Add a scope when it helps (`fix(tui): …`, `feat(container): …`). Breaking changes use `!` (`feat!: …`) or a `BREAKING CHANGE:` footer.
- Keep the summary natural and human — conventional *format*, not robotic wording. No `Co-Authored-By` trailers.
- **[Semantic Versioning](https://semver.org/) is mandatory** for releases/tags (`vMAJOR.MINOR.PATCH`): breaking → major, `feat` → minor, `fix`/`perf` → patch.

## Reporting bugs

Open a [bug report](https://github.com/thereisnotime/x11droid/issues/new?template=bug_report.yml) — the form collects the version, host environment, and logs needed to debug.

## License

By contributing, you agree your contributions are licensed under the project's [GPL-3.0](LICENSE) license.
