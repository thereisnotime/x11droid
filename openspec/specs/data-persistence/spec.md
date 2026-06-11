# Data Persistence Specification

## Purpose
Define how x11droid persists each instance's Android `/data` (and Waydroid state) across restarts, where that data lives on the host, and how leftover ("orphan") data directories — those with no matching container — are reported and pruned to reclaim disk.

## Requirements

### Requirement: Persist instance data in a host volume
The system SHALL persist an instance's Waydroid data by mounting a per-instance host directory into the container at `/var/lib/waydroid`, unless persistence is explicitly disabled. The data directory SHALL live under the invoking (non-root) user's home so it survives container removal.

#### Scenario: Spawning with persistence
- **WHEN** an instance is spawned with persistence enabled
- **THEN** the system creates the instance's data directory and mounts it at `/var/lib/waydroid`

#### Scenario: Disabling persistence
- **WHEN** an instance is spawned with the no-persistent-volume option
- **THEN** no host data directory is mounted and the data is ephemeral

#### Scenario: Data stored under the host user's home
- **WHEN** the data directory path is resolved
- **THEN** it is under `<host-user-home>/.config/x11droid/instances/<name>`, not under root's home

### Requirement: Redirect Android /data into the persistent volume
The entrypoint SHALL redirect Waydroid's `/data` userdata into the mounted volume so installed apps, accounts, and settings survive restarts.

#### Scenario: Linking userdata to the volume
- **WHEN** the entrypoint starts
- **THEN** it symlinks the Waydroid data path to `/var/lib/waydroid/data` so userdata persists in the host volume

### Requirement: Report instance data directories and sizes
The system SHALL list every instance data directory with its on-disk size and whether a matching container still exists.

#### Scenario: Listing data directories
- **WHEN** the user inspects disk usage
- **THEN** each data directory is shown with its size and a flag for whether a container still exists for it

### Requirement: Detect orphan data and nudge to reclaim
The system SHALL identify orphan data directories (those with no matching container) and SHALL surface a one-line nudge when their combined size exceeds a threshold (500 MB).

#### Scenario: Orphan nudge above threshold
- **WHEN** orphan data directories together exceed 500 MB
- **THEN** the system returns a message reporting the count and reclaimable size and pointing the user to prune

#### Scenario: No nudge below threshold
- **WHEN** there are no orphans or their total is below the threshold
- **THEN** no nudge is returned

### Requirement: Prune orphan data directories
The system SHALL delete only the data directories that have no matching container and return the names of those removed, leaving data for existing containers untouched.

#### Scenario: Pruning orphans
- **WHEN** the user prunes orphan data
- **THEN** the system removes each container-less data directory and reports the removed names
- **AND** data directories that still have a container are left intact

### Requirement: Purge an instance's container and data together
The system SHALL provide a purge operation that removes the container and deletes its persistent data directory for a fully clean slate, deleting the data even if the container removal fails.

#### Scenario: Purging an instance
- **WHEN** the user purges an instance
- **THEN** the system removes the container (best-effort) and deletes the instance's data directory

### Requirement: Track installed one-time mods via markers
The system SHALL report, per instance, which one-time mods are installed by reading the marker files the entrypoint writes (libndk, Magisk) and the recorded installed app names.

#### Scenario: Reading instance extras
- **WHEN** instance details are requested
- **THEN** the system reports libndk/Magisk presence from their marker files and lists installed apps from the recorded names
