# Android Extras Specification

## Purpose
Define the optional, per-instance Android customizations x11droid applies on first boot: Google Apps (GApps), ARM translation (libndk), root (Magisk), Android Developer Options, a set of app installers (F-Droid, Aurora Store, Obtainium, Shelter), and a custom device/model name. Each one-time action is gated by a marker file in the instance's data so reboots skip it.

## Requirements

### Requirement: Enable Google Apps on first init
The system SHALL initialize Waydroid with GApps when the user enables it, by passing the GApps system image selection to `waydroid init`.

#### Scenario: Spawning with GApps
- **WHEN** the user spawns an instance with GApps enabled
- **THEN** the container receives `WAYDROID_GAPPS=1` and the entrypoint runs `waydroid init -f -s GAPPS`

#### Scenario: Spawning without GApps
- **WHEN** GApps is not enabled
- **THEN** the entrypoint runs `waydroid init -f` (vanilla image)

### Requirement: Install ARM translation (libndk) once
The system SHALL install the libndk ARM translation layer via waydroid_script when enabled, exactly once per instance, recording completion with a marker file so reboots skip it.

#### Scenario: Installing libndk
- **WHEN** an instance is spawned with ARM translation enabled and the libndk marker is absent
- **THEN** the entrypoint installs libndk and writes the `.x11droid-libndk` marker on success

#### Scenario: Skipping libndk on reboot
- **WHEN** the instance restarts and the libndk marker already exists
- **THEN** the entrypoint does not reinstall libndk

### Requirement: Install Magisk (root) once and register its app
The system SHALL install Magisk via waydroid_script when root is enabled, once per instance, and SHALL register the Magisk manager app with the Android package manager after boot so it opens and reports root.

#### Scenario: Installing Magisk
- **WHEN** an instance is spawned with root enabled and the Magisk marker is absent
- **THEN** the entrypoint installs Magisk and writes the `.x11droid-magisk` marker on success

#### Scenario: Registering the Magisk app after boot
- **WHEN** Magisk is installed and Android has finished booting
- **THEN** the entrypoint installs the bundled Magisk apk so the manager app is registered, recorded by the `.x11droid-magisk-app` marker

### Requirement: Enable Developer Options after boot
The system SHALL enable Android Developer Options and adb after boot when requested, writing the guarded global settings as the shell user so the package check does not fail.

#### Scenario: Enabling Developer Options
- **WHEN** an instance is spawned with Developer Options enabled
- **THEN** after boot the entrypoint sets `development_settings_enabled=1` and `adb_enabled=1`, retrying until the settings service confirms the writes

### Requirement: Install selected apps after boot
The system SHALL install the user-selected apps (F-Droid, Aurora Store, Shelter from the F-Droid repo; Obtainium from GitHub releases) once Android has booted, exactly once per instance, recording each installed app name.

#### Scenario: Spawning with app installers
- **WHEN** the user selects F-Droid and Obtainium
- **THEN** the container receives `WAYDROID_APPS=fdroid,obtainium` and the entrypoint installs each after boot, appending names to `.x11droid-apps`

#### Scenario: No installers selected
- **WHEN** no app installers are selected
- **THEN** the `WAYDROID_APPS` environment variable is omitted and no app installation runs

### Requirement: Set a custom device/model name
The system SHALL set the Android device/model name to a user-provided value (defaulting to the instance name), updating `ro.product.model` in `waydroid_base.prop` and the Settings "Device name" after boot.

#### Scenario: Custom device name provided
- **WHEN** the user provides a device name
- **THEN** the container receives `WAYDROID_DEVICE=<name>`, and the entrypoint rewrites `ro.product.model` and sets the global `device_name` after boot

#### Scenario: Default device name
- **WHEN** no device name is provided
- **THEN** the device name defaults to the instance name
