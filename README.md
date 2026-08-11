# DiscRescue

DiscRescue is a cross-platform, read-only CD/DVD recovery application written in Go 1.26.5+ with a Bubble Tea v2 terminal UI and a single `discrescue` executable.

DiscRescue recovers optical media to an image while maintaining a durable `.drmap` recovery map. Recovery state is tracked independently from image bytes so interrupted or incomplete jobs can be resumed safely and unreadable areas remain explicit instead of being mistaken for recovered data.

Linux, macOS, and Windows use native, platform-specific optical-device implementations behind shared recovery, durability, lifecycle, and UI behavior. See [native platform parity](docs/architecture/native-platform-parity.md) for operation-level details.

## Safety and recovery model

DiscRescue is designed around non-destructive source-media access:

- source optical media is opened read-only;
- image placement preserves the logical sector position of the source media;
- `.drmap` state is authoritative for what has and has not been recovered;
- reads, retries, cancellation, pause, stop, reopen, and shutdown behavior are bounded;
- damaged areas do not require abandoning the rest of the known disc address space;
- incomplete recoveries remain resumable when a valid image and map pair has been established;
- platform and device failures are surfaced as actionable errors rather than silently treated as success.

DiscRescue does not burn, blank, format, repair, decrypt, or otherwise write to the source disc.

## Platform support

| Platform | Discovery and media access | Read-only recovery | Eject implementation |
| --- | --- | --- | --- |
| Linux | Native optical-device discovery using `/dev/sr*` devices | Build-tagged raw optical adapter using the shared bounded recovery engine | Optical-drive ioctl |
| macOS | Pure-Go Darwin disk ioctl discovery and media geometry | Opens the corresponding `/dev/rdiskN` raw device read-only | Bounded `/usr/sbin/diskutil eject` request against the raw device |
| Windows | Win32 optical discovery and geometry | Read-only optical volume adapter | Storage eject device-control request |

The macOS adapter does not require Xcode, Apple SDK headers, or cgo. Eject uses the fixed, bounded `/usr/sbin/diskutil eject` system utility because it coordinates mounted optical volumes; recovery still opens the raw device read-only and never delegates source-media reads to a host command. Raw-device access can require additional host permissions; DiscRescue reports those failures instead of silently falling back to another access path. See [macOS development and hardware validation](docs/architecture/macos-development.md).

Platform compilation and simulator tests do not prove behavior against every physical optical drive or USB/SATA bridge. Hardware-specific claims require validation on representative target hardware.

## Requirements

- Go 1.26.5 or newer when building from source
- A terminal that supports interactive keyboard input
- Permission to read the selected optical device on the host operating system

## Build

From the repository root:

```console
go build -trimpath ./cmd/discrescue
```

Go creates the platform-appropriate executable for the current host.

For metadata-aware release builds, use the repository-owned development tool:

```console
go run ./tools/devtool build --version 0.1.0 --commit dev --build-date 2026-08-11 --output dist/discrescue
```

## Run

Start the terminal UI directly from source:

```console
go run ./cmd/discrescue
```

Or run the built `discrescue` executable using the normal executable-launch convention for the current operating system.

The production executable does not require Gum or a shell helper.

## Recovery methods

The application defaults to **Balanced recovery**.

- **Balanced recovery** uses the complete bounded recovery sequence, including progressively smaller retry work and targeted single-sector attempts before unresolved sectors are finalized.
- **Fast recovery** prioritizes broad coverage with fewer retry stages and leaves unresolved areas available for a later retry instead of spending the full Balanced retry budget immediately.
- **Gentle recovery** uses smaller blocks and lower retry limits than Balanced recovery while preserving resumability for unresolved areas.

All methods use the same durable image and `.drmap` model. Changing recovery method changes the bounded scheduling policy, not the read-only source-media guarantee.

## Resuming and retrying damaged media

DiscRescue keeps recovered, deferred, unreadable, and not-yet-covered areas distinct in recovery state.

When recovery is paused, stopped, interrupted, or finishes with unresolved sectors, a valid image and `.drmap` pair can be used to continue the same recovery. Already recovered sectors remain recorded in the map and do not become missing merely because the image contains placeholder bytes elsewhere.

A later retry operates on the existing recovery state rather than starting an unrelated image from scratch.

## Deterministic development inputs

The repository includes deterministic discovery and fault-injection facilities for development and testing. These are testing tools, not substitutes for native or physical-hardware validation.

A controlled optical-drive list can be supplied through `DISKRESCUE_DISCOVERY_DRIVES` using this value format:

```text
path|display name|status;path|display name|status
```

Example value:

```text
/dev/testdvd|Synthetic DVD drive|ready;/dev/testcd|Synthetic CD drive|empty
```

Set the variable with the operating system, IDE, or terminal environment mechanism appropriate to the host, then run:

```console
go run ./cmd/discrescue
```

Development code can also force discovery error or unsupported-environment paths with `DISKRESCUE_DISCOVERY_ERROR` and `DISKRESCUE_DISCOVERY_UNSUPPORTED`.

## Test and verification

The repository-owned Go devtool is the canonical shell-neutral validation interface for Linux, macOS, and Windows.

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

The release command reports unsupported or unavailable race validation as `SKIPPED`; an explicit `--race=on` makes that condition a failure.

Validation evidence has different strengths:

- portable tests cover shared logic, TUI behavior, formats, simulation, and recovery planning;
- native validation covers the target operating system and selected platform implementation;
- cross-compilation supplements native validation but does not replace it;
- physical optical-drive claims require real target-hardware evidence.

See [validation workflow](docs/architecture/validation-workflow.md) for the complete validation contract.

## Project layout

- `cmd/discrescue` - executable entrypoint
- `internal/app` - Bubble Tea application model, update, view, keys, and runtime effect wiring
- `internal/recovery` - shared recovery methods, policies, scheduling, and bounded read behavior
- `internal/platform` - project-owned runtime adapters and native optical capability boundaries
- `internal/device` - device capability, protocol, media, and worker-facing types
- `internal/image` - positioned image output behavior
- `internal/mapfile` - durable `.drmap` recovery state, replay, and checkpoints
- `internal/catalog` - local processed-media identity and history state
- `internal/integrity` - verification and recovery-confidence boundaries
- `internal/merge` - conservative multi-capture merge behavior
- `internal/testdevice` - deterministic device and failure simulation
- `tools/devtool` - cross-platform repository validation and build tool
- `docs/architecture` - architecture, platform, and validation documentation
- `docs/adr` - architectural decision records
- `docs/formats` - versioned project-owned format specifications

## Key repository documents

- [AGENTS.md](AGENTS.md) - contributor and engineering contract
- [Cross-platform development](docs/architecture/cross-platform-development.md)
- [Native platform parity](docs/architecture/native-platform-parity.md)
- [Validation workflow](docs/architecture/validation-workflow.md)
- [macOS development and hardware validation](docs/architecture/macos-development.md)
- [DiscRescue recovery map format v1](docs/formats/drmap-v1.md)
