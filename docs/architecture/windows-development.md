# Windows Development Contract

DiscRescue is being developed on Windows first, but the product target remains Linux for optical-device access. Local Windows work must focus on simulator-backed behavior, pure transitions, durable format code, and the Bubble Tea interface.

## What Must Work on Windows

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`
- Bubble Tea shell iteration in PowerShell or Windows Terminal
- PowerShell repository scripts under `scripts/`

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

## Assumption

The bootstrap module path is `discrescue`. If the repository later receives a canonical remote import path, the module path can be updated as a separate change.
