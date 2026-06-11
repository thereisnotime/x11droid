# Kernel Binder Specification

## Purpose
Define how x11droid ensures the host kernel provides Android's `binder` IPC. Android runs natively on the host kernel, so the `binder_linux` module must be loaded; `ashmem_linux` is optional (built-in on kernels >= 5.18). Because this kernel is built with `CONFIG_ANDROID_BINDER_DEVICES=""`, the container provisions its own binder device nodes via binderfs at runtime rather than relying on static host nodes.

## Requirements

### Requirement: Load the required binder module
The system SHALL load `binder_linux` via `modprobe` when it is not already present, running as root (no sudo prefix) since x11droid runs under sudo.

#### Scenario: Loading binder_linux when absent
- **WHEN** modules are loaded and `binder_linux` is not in `/proc/modules`
- **THEN** the system runs `modprobe binder_linux` and returns an error if it fails

#### Scenario: Skipping load when already present
- **WHEN** `binder_linux` is already loaded
- **THEN** the system does not invoke `modprobe binder_linux` again

### Requirement: Treat ashmem_linux as optional
The system SHALL attempt to load `ashmem_linux` but SHALL NOT fail if it is absent, since kernels >= 5.18 ship it built-in and `modprobe` reports "not found".

#### Scenario: ashmem load failure ignored
- **WHEN** modules are loaded and `modprobe ashmem_linux` fails
- **THEN** the system ignores the failure and reports overall success

### Requirement: Report module status
The system SHALL report the state of each tracked module as loaded, built-in/optional, or missing, distinguishing the required module from the optional one.

#### Scenario: Status of required and optional modules
- **WHEN** module status is queried
- **THEN** `binder_linux` is reported as loaded or missing, and `ashmem_linux` is reported as loaded or optional

#### Scenario: All-loaded check ignores optional modules
- **WHEN** the all-loaded check runs
- **THEN** it passes as long as every required module is OK, regardless of the optional module's state

### Requirement: Unload modules in reverse order
The system SHALL unload the tracked modules in reverse order via `rmmod`, treating "not currently loaded" or "not found" as non-fatal.

#### Scenario: Unloading modules
- **WHEN** the user unloads modules
- **THEN** the system runs `rmmod` for each module from last to first
- **AND** a module that is not loaded does not cause an error

### Requirement: Provision binder nodes via binderfs in the container
Because the host kernel creates no static binder nodes, the container SHALL mount binderfs and allocate the binder/vndbinder/hwbinder nodes via the `BINDER_CTL_ADD` ioctl, using the device names from `waydroid.cfg` so gbinder can open the expected paths.

#### Scenario: Allocating binder nodes at boot
- **WHEN** the entrypoint runs and `/dev/<binder-node>` does not exist
- **THEN** it mounts binderfs and issues `BINDER_CTL_ADD` for the binder, vndbinder, and hwbinder node names, symlinking and chmod 0666-ing them

#### Scenario: Honoring configured device names
- **WHEN** `waydroid.cfg` specifies a binder device name (e.g. `anbox-binder`)
- **THEN** the entrypoint provisions that exact name rather than a hard-coded default
