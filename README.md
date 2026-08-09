# DiscRescue

DiscRescue is a Go 1.26.5+ optical-disc recovery application with a Bubble Tea v2 terminal UI and a single `discrescue` executable. The repository uses a simulator-first workflow and supports Linux, macOS, and Windows development; production device access remains platform-specific and read-only. Operation-level support is exposed through project-owned capability types; see [native platform parity](docs/architecture/native-platform-parity.md).

macOS optical-drive access uses pure-Go Darwin disk ioctls for device discovery, media geometry, and eject, then opens the corresponding `/dev/rdiskN` device read-only for recovery. The macOS adapter is built with `CGO_ENABLED=0` and does not require Xcode, a macOS SDK, or host utilities. macOS may require additional permissions for raw-device access; DiscRescue reports that failure. See [macOS development and hardware validation](docs/architecture/macos-development.md) for the manual USB-drive check.

## Requirements

- Go 1.26.5 or newer
- A terminal that supports interactive keyboard input

## Build

Build the default executable:

```console
go build -trimpath ./cmd/discrescue
```

Build a release-style binary with embedded metadata:

```console
go run ./tools/devtool build --version 0.1.0 --commit dev --build-date 2026-08-06 --output dist/discrescue
```

## Run

Start the terminal UI:

```console
go run ./cmd/discrescue
```

Run the built executable:

```console
.\discrescue.exe
```

### Optional demo workflow

The production executable does not require a shell helper or Gum. Run
`go run ./cmd/discrescue` directly; controlled discovery inputs can be supplied
through the environment as described below.

## Recovery behavior

DiscRescue now defaults to `Fast recovery`.

- `Fast recovery` skips damaged ranges during the fast pass and stops after easy data is covered so you can decide whether to retry deferred sectors later.
- `Continue through retry pass` finishes the fast pass and then immediately starts the deferred-sector retry pass.

The active recovery view shows disc coverage for the current pass so progress remains meaningful even when damaged areas are being deferred instead of recovered immediately. When the fast pass reaches a damaged band, it jumps ahead by a bounded amount and probes for the next readable area instead of testing every nearby cluster during the first pass.

## Controlled startup workflow

You can provide a controlled set of optical drives for local validation by setting `DISKRESCUE_DISCOVERY_DRIVES`.

Format:

```text
path|display name|status;path|display name|status
```

Example:

```console
go run ./cmd/discrescue
```

Set `DISKRESCUE_DISCOVERY_DRIVES` in the environment using the operating
system's normal environment settings before launching the command.

This is useful for verifying the startup workflow when a physical optical drive is not available on the current machine.

You can also force specific discovery outcomes:

```console
go run ./cmd/discrescue
```

```console
go run ./cmd/discrescue
```

## Test and verification

Apply formatting:

```console
go run ./tools/devtool format
```

Check formatting without changing files:

```console
go run ./tools/devtool format --check
```

Run the normal validation gate:

```console
go run ./tools/devtool check
```

Run the full local release gate:

```console
go run ./tools/devtool release --race=auto
```

The release command reports unsupported or unavailable race validation as
`SKIPPED`; an explicit `--race=on` makes that condition a failure.

## Project layout

- `cmd/discrescue` - executable entrypoint and worker mode
- `internal/app` - Bubble Tea model, update, view, and runtime effect wiring
- `internal/platform` - project-owned runtime adapters and optical-drive discovery
- `docs/architecture` - architecture and validation notes
- `docs/formats` - versioned project-owned formats

## Key repository documents

- [AGENTS.md](AGENTS.md)
- [docs/architecture/package-layout.md](docs/architecture/package-layout.md)
- [docs/architecture/cross-platform-development.md](docs/architecture/cross-platform-development.md)
- [docs/architecture/validation-workflow.md](docs/architecture/validation-workflow.md)
- [docs/formats/drmap-v1.md](docs/formats/drmap-v1.md)
