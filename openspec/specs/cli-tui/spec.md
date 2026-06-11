# CLI and TUI Specification

## Purpose
Define the user-facing interface of x11droid: the scriptable Cobra subcommands, the root-required gating, and the interactive Bubble Tea TUI (dashboard, instance detail actions, spawn form, setup menu, config view) including its prerequisite and orphan-data warnings. A bare invocation launches the TUI; subcommands are for scripting.

## Requirements

### Requirement: Provide scriptable subcommands
The system SHALL expose subcommands for managing instances: `list` (aliases `ls`/`ps`), `spawn`, `attach`, `hide`, `start`, `stop`, `rm` (aliases `remove`/`delete`), `logs`, `shell`, `adb` (aliases `android-shell`/`ashell`), `install` (alias `apk`), `logcat`, `config`, `prune`, `setup`, and `version`.

#### Scenario: Listing instances from the CLI
- **WHEN** the user runs `x11droid list`
- **THEN** the system prints a table of instances with NAME, ID, STATUS, and IMAGE columns

#### Scenario: Spawning from the CLI with flags
- **WHEN** the user runs `x11droid spawn <name>` with flags such as `--gapps`, `--hidearm`, `--root`, `--dev-options`, `--fdroid`, `--aurora`, `--obtainium`, `--shelter`, `--device-name`, or `--no-pv`
- **THEN** the system spawns the instance with the corresponding options applied

#### Scenario: Removing with optional purge
- **WHEN** the user runs `x11droid rm <name>` with `--purge`
- **THEN** the system removes the container and deletes its persisted data; without `--purge` it removes only the container

### Requirement: Launch the TUI on bare invocation
The system SHALL launch the interactive Bubble Tea TUI when invoked with no subcommand, in the alternate screen with mouse support.

#### Scenario: Bare invocation
- **WHEN** the user runs `x11droid` with no subcommand
- **THEN** the TUI dashboard starts

### Requirement: Gate privileged commands on root
The system SHALL require root for commands that touch podman/waydroid and SHALL refuse early with a clear message, while leaving read-only commands (`version`, `help`, `status`, `config`) usable without root.

#### Scenario: Privileged command without root
- **WHEN** a non-root user runs a command that needs podman
- **THEN** the system prints `error: x11droid must run as root — try: sudo x11droid` and exits non-zero

#### Scenario: Read-only command without root
- **WHEN** a non-root user runs `version`, `help`, `status`, or `config`
- **THEN** the command runs without requiring root

### Requirement: Open interactive in-container sessions
The system SHALL provide interactive sessions inside an instance: a container bash shell, an Android root shell (`waydroid shell`), and Android logcat, each refusing with a clear error when podman is missing or the instance is not running.

#### Scenario: Opening a container shell
- **WHEN** the user runs `x11droid shell <name>` on a running instance
- **THEN** the system execs `podman exec -it <name> bash`

#### Scenario: Opening the Android shell
- **WHEN** the user runs `x11droid adb <name>` on a running instance
- **THEN** the system execs an interactive `waydroid shell` inside the container

#### Scenario: Session refused when not runnable
- **WHEN** podman is missing or the instance is not running
- **THEN** the command returns a clear error instead of execing

### Requirement: Install a local APK into an instance
The system SHALL install a local `.apk` into a running instance by copying it in and running `waydroid app install`.

#### Scenario: Installing an APK
- **WHEN** the user runs `x11droid install <name> <file.apk>` and the file exists
- **THEN** the system copies the apk into the container and installs it via `waydroid app install`

#### Scenario: Missing APK file
- **WHEN** the apk path does not exist or is a directory
- **THEN** the system returns an "apk not found" error

### Requirement: Provide instance detail actions in the TUI
The TUI detail view SHALL offer the actions Show UI, Hide UI, Start, Stop, Remove, Purge, Shell, Android Shell, Logs, and Logcat for the selected instance.

#### Scenario: Triggering Show UI from the detail view
- **WHEN** the user selects Show UI for an instance
- **THEN** the TUI opens (or recovers) the Android window for that instance

#### Scenario: Destructive action confirmation
- **WHEN** the user selects Remove or Purge
- **THEN** the TUI prompts for confirmation before performing the action

### Requirement: Provide a setup menu in the TUI
The TUI setup view SHALL offer Load Modules, Unload Modules, Build Image, Prune Orphan Data, and Refresh, prompting for confirmation before pruning.

#### Scenario: Building the image from setup
- **WHEN** the user selects Build Image
- **THEN** the TUI suspends and runs the image build so its output is visible

#### Scenario: Pruning from setup
- **WHEN** the user selects Prune Orphan Data
- **THEN** the TUI confirms, then deletes orphan data directories

### Requirement: Warn about missing prerequisites
The TUI SHALL display a prerequisite warning when required setup is incomplete: not running as root, podman missing or not responding, the image not built, or a required kernel module not loaded.

#### Scenario: Not root
- **WHEN** the TUI runs without root
- **THEN** it warns to quit and restart with `sudo x11droid`

#### Scenario: Missing setup components
- **WHEN** podman is missing, the image is not built, or `binder_linux` is not loaded
- **THEN** the TUI lists the issues and directs the user to open Setup

### Requirement: Warn about reclaimable orphan data
The TUI dashboard SHALL display the orphan-data nudge when orphan directories exceed the size threshold.

#### Scenario: Orphan nudge on the dashboard
- **WHEN** orphan data exceeds the threshold
- **THEN** the dashboard shows the reclaimable-space nudge

### Requirement: Show and persist instance defaults via config
The system SHALL show and set instance defaults — resolution (width/height), orientation, and compositor — persisting them to a config file, and SHALL display the effective window dimensions.

#### Scenario: Setting a default
- **WHEN** the user runs `x11droid config --width 720 --height 1280`
- **THEN** the system saves the values and prints the resolution, orientation, compositor, and effective window dimensions

#### Scenario: Showing config
- **WHEN** the user runs `x11droid config` with no flags
- **THEN** the system prints the current defaults without modifying them
