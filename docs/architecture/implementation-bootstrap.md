# DiscRescue Implementation Bootstrap

This repository is bootstrapped from the project-owned TDD in `.project/discrescue-tdd.md`. The TDD is the normative contract. Current code is the initial scaffold for package ownership, compile boundaries, and a cross-platform development loop.

## Constraints

- DiscRescue remains Linux-first for production optical-device access.
- macOS, Linux, and Windows are supported development platforms for simulator work, pure transitions, documentation, and the Bubble Tea shell.
- Device behavior must be implemented only from public OS, SCSI, and MMC specifications.
- Project-owned formats, protocols, and messages must be documented under `docs/formats/` before they are treated as stable.

## Public Specification Inputs

- Go 1.26.5+ language and standard library.
- `charm.land/bubbletea/v2` for the TUI runtime boundary.
- Public Linux block-device and `SG_IO` APIs for later device adapters.
- Public MMC and SCSI command specifications for later command and sense handling.

## Current Bootstrap Scope

- `cmd/discrescue` contains the single-executable entrypoint and a placeholder worker mode switch.
- `internal/app` contains the initial TEA-style Bubble Tea model with deterministic update and pure view behavior.
- `internal/coordinator` contains a minimal pure transition boundary for job state changes and emitted effects.
- The remaining internal packages are stubs that define ownership and vocabulary without pretending the durable or hardware paths are complete.

## Next Required Documents

- `docs/formats/drmap-v1.md`
- `docs/formats/worker-protocol.md`
- `docs/formats/content-identity-v1.md`
- `docs/formats/catalog-v1.md`
- `docs/formats/event-log.md`

## Release Notes

- `docs/architecture/linux-release-packaging.md`
- `docs/architecture/package-layout.md`
