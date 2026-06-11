# Instance Lifecycle Specification

## Purpose
Define how x11droid creates, starts, stops, and removes Waydroid instances as rootful Podman containers. Each instance is a privileged container that runs Waydroid (Android on the host kernel) with the host IPC and cgroup namespaces shared but its own network namespace, and is tracked by the `x11droid=true` Podman label.

## Requirements

### Requirement: Spawn a privileged instance container
The system SHALL create a new instance by running `podman run -d` with `--name <name>`, the `x11droid=true` label, and `--privileged`, using the `x11droid:latest` image.

#### Scenario: Spawning a new instance
- **WHEN** the user spawns an instance named `phone`
- **THEN** the system runs `podman run -d --name phone --label x11droid=true --privileged ... x11droid:latest`
- **AND** the container is detached and tracked by the `x11droid=true` label

#### Scenario: Spawn failure surfaces podman output
- **WHEN** `podman run` exits non-zero
- **THEN** the system returns an error that includes the combined podman output

### Requirement: Use host IPC and cgroup namespaces with an isolated network namespace
The container SHALL run with `--ipc=host` and `--cgroupns=host`, and SHALL NOT use `--network=host`, so Waydroid can create its own `waydroid0` bridge in a private network namespace while sharing the host IPC and cgroup namespaces.

#### Scenario: Namespace flags on spawn
- **WHEN** an instance is spawned
- **THEN** the run arguments include `--ipc=host` and `--cgroupns=host`
- **AND** the run arguments do NOT include `--network=host`

#### Scenario: PID limit lifted for Android
- **WHEN** an instance is spawned
- **THEN** the run arguments include `--pids-limit=-1` so Android's thread count is not capped at the podman default

### Requirement: Start a stopped instance with cleanup-aware retry
The system SHALL start a stopped instance via `podman start`, retrying with a short backoff because runc/conmon may still be tearing down a previous run's cgroup and network namespace.

#### Scenario: Start succeeds on first attempt
- **WHEN** the user starts a stopped instance and podman succeeds
- **THEN** `podman start <name>` runs once and the call returns success

#### Scenario: Start retries while teardown finishes
- **WHEN** `podman start` initially fails because the prior run's namespaces are still being released
- **THEN** the system retries several times with a delay between attempts before returning the final error

### Requirement: Stop a running instance with a short grace period
The system SHALL stop a running instance via `podman stop` with a reduced timeout so the container does not linger in the "Stopping" state.

#### Scenario: Stopping an instance
- **WHEN** the user stops a running instance
- **THEN** the system runs `podman stop -t 5 <name>`

### Requirement: Remove an instance by force
The system SHALL remove an instance with `podman rm -f -t 0` so a container hung in "Stopping" (loop mounts / LXC not tearing down) is force-killed immediately rather than waiting indefinitely.

#### Scenario: Removing an instance
- **WHEN** the user removes an instance
- **THEN** the system runs `podman rm -f -t 0 <name>`

### Requirement: List instances and their running state
The system SHALL list all instances by querying `podman ps -a --filter label=x11droid=true --format json`, exposing each instance's name, short ID, status, and image, and SHALL be able to filter the list to only running instances.

#### Scenario: Listing all instances
- **WHEN** the user lists instances
- **THEN** the system parses the podman JSON and returns one entry per `x11droid=true` container with name, 12-char ID, status, and image

#### Scenario: Empty list when none exist
- **WHEN** no `x11droid=true` containers exist
- **THEN** the system returns an empty list rather than an error

#### Scenario: Filtering to running instances
- **WHEN** the running instances are requested
- **THEN** only instances whose status begins with "Up" are returned

### Requirement: Report per-instance runtime memory usage
The system SHALL report each running instance's memory usage from `podman stats --no-stream`, using a single batched call when building the dashboard list, and SHALL show no usage for instances that are not running.

#### Scenario: Memory shown for a running instance
- **WHEN** the instance list is built and a container is running
- **THEN** its memory usage is read from the batched `podman stats` output

#### Scenario: No memory for a stopped instance
- **WHEN** an instance is not running
- **THEN** its memory column is reported as unavailable
