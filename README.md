# DiscRescue

DiscRescue is a Go 1.25+ optical-disc recovery application with a Bubble Tea v2 terminal UI and a single `discrescue` executable. The current repository focuses on a simulator-first, Windows-validated startup workflow and Linux-first production device access.

## Requirements

- Go 1.25 or newer
- PowerShell on Windows for the provided scripts
- Windows Terminal or another terminal that supports interactive keyboard input

## Build

Build the default executable:

```powershell
go build -trimpath ./cmd/discrescue
```

Build a release-style Windows binary with embedded metadata:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1 `
  -Version 0.1.0 `
  -Commit dev `
  -BuildDate 2026-08-06 `
  -Output dist/discrescue.exe
```

## Run

Start the terminal UI:

```powershell
go run ./cmd/discrescue
```

Run the built executable:

```powershell
.\discrescue.exe
```

## Recovery behavior

DiscRescue now defaults to `Fast recovery`.

- `Fast recovery` skips damaged ranges during the fast pass and stops after easy data is covered so you can decide whether to retry deferred sectors later.
- `Continue through retry pass` finishes the fast pass and then immediately starts the deferred-sector retry pass.

The active recovery view shows disc coverage for the current pass so progress remains meaningful even when damaged areas are being deferred instead of recovered immediately.

## Controlled startup workflow

You can provide a controlled set of optical drives for local validation by setting `DISKRESCUE_DISCOVERY_DRIVES`.

Format:

```text
path|display name|status;path|display name|status
```

Example:

```powershell
$env:DISKRESCUE_DISCOVERY_DRIVES = 'D:\|Virtual DVD Drive|available;E:\|Physical Blu-ray Drive|available'
go run ./cmd/discrescue
```

This is useful for verifying the startup workflow when a physical optical drive is not available on the current machine.

You can also force specific discovery outcomes:

```powershell
$env:DISKRESCUE_DISCOVERY_UNSUPPORTED = '1'
go run ./cmd/discrescue
```

```powershell
$env:DISKRESCUE_DISCOVERY_ERROR = 'permission denied while listing optical drives'
go run ./cmd/discrescue
```

## Test and verification

Format changed files:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/format.ps1
```

Run tests:

```powershell
go test ./...
```

Run vet and build checks:

```powershell
go vet ./...
go build -trimpath ./cmd/discrescue
```

Run the local release gate:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/release-gates.ps1
```

## Project layout

- `cmd/discrescue` - executable entrypoint and worker mode
- `internal/app` - Bubble Tea model, update, view, and runtime effect wiring
- `internal/platform` - project-owned runtime adapters and optical-drive discovery
- `docs/architecture` - architecture and validation notes
- `docs/formats` - versioned project-owned formats

## Key repository documents

- [AGENTS.md](AGENTS.md)
- [docs/architecture/package-layout.md](docs/architecture/package-layout.md)
- [docs/architecture/validation-workflow.md](docs/architecture/validation-workflow.md)
- [docs/formats/drmap-v1.md](docs/formats/drmap-v1.md)
