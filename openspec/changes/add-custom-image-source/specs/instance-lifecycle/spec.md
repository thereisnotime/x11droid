# Instance Lifecycle Specification

## ADDED Requirements

### Requirement: Spawn from a custom system/vendor image

The system SHALL allow spawning an instance from a custom system and vendor image instead of the official Waydroid download, when BOTH `--system-image` and `--vendor-image` are provided. Each source MAY be a local path or an `http(s)` URL, and either a raw `.img` or a `.zip` containing it.

#### Scenario: Both image sources provided

- **WHEN** the user spawns with `--system-image <src>` and `--vendor-image <src>`
- **THEN** the system resolves each source — downloading it if it is a URL and extracting the matching `.img` if it is a `.zip`
- **AND** writes the two images to `~/.config/x11droid/extra-images/<name>/`
- **AND** mounts that directory at `/etc/waydroid-extra/images` so `waydroid init` uses them instead of downloading the official image

#### Scenario: Only one image source provided

- **WHEN** the user provides only one of `--system-image` / `--vendor-image`
- **THEN** the system returns an error stating that both are required

#### Scenario: Invalid image rejected

- **WHEN** a resolved image is below the minimum size (a truncated download) or is a still-sparse Android image
- **THEN** the system returns an error describing the problem instead of using the image
