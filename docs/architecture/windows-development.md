# Windows Development Contract

DiscRescue is being developed on Windows first, but the product target remains Linux for optical-device access. Local Windows work must focus on simulator-backed behavior, pure transitions, durable format code, and the Bubble Tea interface.

## What Must Work on Windows

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`
- Bubble Tea shell iteration in PowerShell or Windows Terminal
- PowerShell repository scripts under `scripts/`
- cross-platform runtime adapters under `internal/platform`

## What Stays Out of Scope on Windows

- Real Linux block-device access
- Real `SG_IO` optical-device commands
- Any shortcut that weakens read-only media guarantees

## Path and Shell Rules

- Use PowerShell commands in developer docs and scripts.
- Keep paths relative to the repository root where possible.
- Avoid shell-specific behavior that assumes Bash on Windows.

## Initial Workflow

1. Run `scripts/format.ps1`.
2. Run `scripts/test.ps1`.
3. Run `scripts/check.ps1`.
4. Run `go run ./cmd/discrescue`.

## Windows Terminal TUI Verification

Use Windows Terminal as the primary manual TUI check on Windows. Verify these behaviors against the current Bubble Tea shell:

- keyboard-only navigation remains usable on the main flow, recovery flow, paused flow, stop confirmation, and summary screens;
- resize from `80x24` to `60x18`, then to `40x12`, without restarting or cancelling the active recovery screen;
- below `40x12`, the TUI shows a resize request and preserves the current recovery activity;
- monochrome-safe rendering stays readable without relying on color-only distinctions;
- long archive paths wrap instead of forcing horizontal scrolling;
- alternate-screen pages restore correctly when leaving recovery or details views.

The automated baseline for these checks is the app test suite, especially the view tests for:

- `80x24`, `60x18`, and `40x12` layouts;
- below-minimum resize handling;
- monochrome-safe progress rendering;
- wrapped output paths;
- recovery alt-screen preservation during resize.

## Assumption

The bootstrap module path is `discrescue`. If the repository later receives a canonical remote import path, the module path can be updated as a separate change.
