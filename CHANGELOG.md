# Changelog

## [0.1.1](https://github.com/thereisnotime/x11droid/compare/v0.1.0...v0.1.1) (2026-06-11)


### Features

* add adb/install/logcat to CLI+TUI, warn on missing/broken podman at startup ([771d6c7](https://github.com/thereisnotime/x11droid/commit/771d6c78a2ef733fb55ffa60066c5eb8a804180a))


### Bug Fixes

* migrate .golangci.yml exclude-rules to v2 linters.exclusions (CI lint config verify) ([2020265](https://github.com/thereisnotime/x11droid/commit/202026593de07b9b3fec71154b5dbcaabc985fa2))
* set WLR_BACKENDS=x11, WLR_RENDERER=pixman, XDG_SESSION_TYPE=x11 ([94509d0](https://github.com/thereisnotime/x11droid/commit/94509d0571fb40d3f4a7dbf3cad85e466c9e4d22))


### Dependencies

* add yamllint + actionlint (just recipes, .tool-versions, .yamllint config) ([475a945](https://github.com/thereisnotime/x11droid/commit/475a9457a926c01b7661de1521313621555131ec))
* use err==nil instead of container.ImageExists() in the ExecProcess ([e6d871d](https://github.com/thereisnotime/x11droid/commit/e6d871db90d6321dd5df280133bd6452ffbdcb81))

## Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This file is maintained automatically by [release-please](https://github.com/googleapis/release-please).
