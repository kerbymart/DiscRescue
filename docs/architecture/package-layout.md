# DiscRescue Package Layout

This note records the repository package scaffold and its ownership boundary relative to TDD Section 14.

## Runtime entrypoint

- `cmd/discrescue`: single executable entrypoint and internal worker-mode switch.

## Internal packages

| Package | Responsibility |
| --- | --- |
| `internal/app` | Bubble Tea model, messages, update, view, keys, and presentation-only projections |
| `internal/buildinfo` | Embedded version, commit, and build-date metadata shown by the app |
| `internal/catalog` | Logical-content identity lookup, local journal, snapshot, and bounded history state |
| `internal/coordinator` | Authoritative job transitions, checkpointing, cancellation, and shutdown |
| `internal/device` | Worker protocol, supervisor, media model, and bounded command handling |
| `internal/device/linux` | Linux block-device and `SG_IO` adapters plus MMC/CDB encoding |
| `internal/image` | Positioned image writes and ISO/BIN/CUE output behavior |
| `internal/integrity` | Verification boundaries, confidence updates, and provenance evidence |
| `internal/logging` | Bounded structured event records and redaction policy |
| `internal/mapfile` | `.drmap` header, checkpoint, journal, replay, and extent state |
| `internal/merge` | Conservative multi-capture merge planning and selected-output provenance |
| `internal/platform` | Project-owned filesystem, clock, process, and terminal adapters |
| `internal/recovery` | Deterministic probe, fast, trim, adaptive, scrape, and completion scheduling |
| `internal/testdevice` | Deterministic simulator scenarios, command audit, and release hardening checks |

## Boundary rules

- Production device behavior stays behind `internal/device` and `internal/device/linux`.
- The Bubble Tea layer depends on coordinator projections, not direct device backends.
- Persisted formats stay owned by `internal/mapfile`, `internal/catalog`, and the documents under `docs/formats/`.
- Cross-platform development orchestration stays in `tools/devtool`; runtime platform adapters remain in `internal/platform` so simulator and TUI work do not weaken Linux production boundaries.

## Verification

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`
