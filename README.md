# DiscRescue

DiscRescue is a Go 1.26.5 optical-disc recovery application with a Bubble Tea v2 terminal UI and a single `discrescue` executable. Shared application, simulator, format, and TUI code is developed and validated on macOS, Linux, and Windows; physical optical-device behavior remains hardware-dependent.

macOS optical-drive access uses the system `diskutil` command for discovery and media geometry, then opens the corresponding `/dev/rdiskN` device read-only for recovery. macOS may require additional permissions for raw-device access; DiscRescue reports that failure and does not unmount or eject media automatically. See [macOS development and hardware validation](docs/architecture/macos-development.md) for the manual USB-drive check.

## Requirements

- Go 1.26.5 or newer
- A terminal that supports interactive keyboard input

## Build

Build the default executable:

```console
go build -trimpath ./cmd/discrescue
```

Build a release-style executable with embedded metadata:

```console
go run ./tools/devtool build \
  --version 0.1.0 \
  --commit dev \
  --build-date 2026-08-09 \
  --output dist/discrescue
```

## Run

Start the terminal UI:

```console
go run ./cmd/discrescue
```

Run the built executable:

```console
./discrescue
```

### Optional demo workflow

The production executable does not require Gum. On Unix-like systems, install
Gum optionally and run `scripts/demo.sh` to choose a controlled synthetic-drive
scenario, build, launch, and optionally inspect or clean up demo artifacts.
Without Gum, run `go run ./cmd/discrescue` directly. The companion
`scripts/inspect-log.sh` command reads logs from a file and never writes into
the active Bubble Tea renderer. These are optional convenience workflows, not
the canonical validation interface.

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
DISKRESCUE_DISCOVERY_DRIVES='D:\|Virtual DVD Drive|available;E:\|Physical Blu-ray Drive|available' go run ./cmd/discrescue
```

This is useful for verifying the startup workflow when a physical optical drive is not available on the current machine.

You can also force specific discovery outcomes:

```console
DISKRESCUE_DISCOVERY_UNSUPPORTED=1 go run ./cmd/discrescue
```

```console
DISKRESCUE_DISCOVERY_ERROR='permission denied while listing optical drives' go run ./cmd/discrescue
```

## Test and verification

Format all Go files:

```console
go run ./tools/devtool format
```

Run tests:

```console
go run ./tools/devtool test
```

Run the normal local validation contract:

```console
go run ./tools/devtool check
```

Run release-oriented validation, including simulator, command-audit, leak, benchmark, and race gates when supported:

```console
go run ./tools/devtool release --race=auto
```

The same release command runs natively on `ubuntu-latest`, `macos-latest`, and
`windows-latest` in GitHub Actions. Hardware-dependent optical-drive checks
remain separate from portable CI validation.

## Project layout

- `cmd/discrescue` - executable entrypoint and worker mode
- `internal/app` - Bubble Tea model, update, view, and runtime effect wiring
- `internal/platform` - project-owned runtime adapters and optical-drive discovery
- `tools/devtool` - shell-neutral formatting, testing, validation, and build commands
- `docs/architecture` - architecture and validation notes
- `docs/formats` - versioned project-owned formats

## Key repository documents

- [AGENTS.md](AGENTS.md)
- [docs/architecture/cross-platform-development.md](docs/architecture/cross-platform-development.md)
- [docs/architecture/package-layout.md](docs/architecture/package-layout.md)
- [docs/architecture/validation-workflow.md](docs/architecture/validation-workflow.md)
- [docs/formats/drmap-v1.md](docs/formats/drmap-v1.md)
