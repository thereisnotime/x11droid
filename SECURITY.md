# Security Policy

## Reporting a vulnerability

Please report security issues **privately** via [GitHub Security Advisories](https://github.com/thereisnotime/x11droid/security/advisories/new) rather than opening a public issue. I'll respond as soon as I can.

## Threat model note

x11droid runs **rootful** (`sudo x11droid`) and spawns **privileged** containers with host kernel access (`binder`) — waydroid requires this; it can't run rootless. Treat the instances, images, and the apps you install in them as trusted. Don't run untrusted Android images, and don't put secrets you care about into an instance.

## Supported versions

Under active development — only the latest `master` is supported.
