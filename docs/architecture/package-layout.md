# DiscRescue Package Layout

This note records the repository package scaffold and its ownership boundary relative to TDD Section 14.

## Runtime entrypoint

- `cmd/discrescue`: single executable entrypoint and internal worker-mode switch.

## Development tooling

- `tools/devtool`: repository validation/build command dispatcher; each
  format, check, release, build, file-discovery, runner, and race-policy owner
  lives in a focused same-package file.
- `tools/macos-eject-probe`: opt-in macOS eject diagnostic with its own bounded
  native request and command-line boundary; production packages do not depend
  on it.
- `scripts/demo.sh`: TUI demonstration workflow.
- `scripts/inspect-log.sh`: bounded diagnostic-log inspection workflow.

## Internal packages

| Package | Responsibility |
| --- | --- |
| `internal/app` | Bubble Tea model, messages, update, navigation, presentation, and runtime effect projections; `layout.go` owns geometry, while screen, compact-rendering, runtime, and workflow responsibilities stay in focused same-package files |
| `internal/buildinfo` | Embedded version, commit, and build-date metadata shown by the app |
| `internal/catalog` | Logical-content identity lookup, local journal, snapshot, and bounded history state |
| `internal/coordinator` | Authoritative job transitions, checkpointing, cancellation, and shutdown |
| `internal/device` | Worker protocol, supervisor, media model, and bounded command handling; frame/payload codecs and deadline/restart policy are separate same-package owners |
| `internal/device/linux` | Linux block-device and `SG_IO` adapters plus MMC/CDB encoding |
| `internal/image` | Positioned image writes and ISO/BIN/CUE output behavior |
| `internal/integrity` | Verification boundaries, confidence updates, provenance evidence, and separate map/image/external workflows |
| `internal/logging` | Bounded structured event records and redaction policy |
| `internal/mapfile` | Pure `.drmap` header, checkpoint, journal, replay, extent model/operations, and geometry rules |
| `internal/merge` | Conservative multi-capture merge planning, candidate analysis, coalescing, and selected-output provenance |
| `internal/platform` | Project-owned filesystem, clock, process, terminal, optical, and native recovery adapters; recovery job lifecycle, execution, source, output, target, and map responsibilities remain separated in build-tagged same-package files |
| `internal/recovery` | Deterministic probe, fast, trim, adaptive, scrape, and completion scheduling |
| `internal/recoverymap` | Durable runtime `.drmap` lifecycle, including create/open/inspect, staged append/flush, close, and exact file I/O |
| `internal/testdevice` | Deterministic simulator scenario definitions/validation, eject behavior, command audit, and release hardening checks |

## Boundary rules

- Production device behavior stays behind `internal/device` and `internal/device/linux`.
- The Bubble Tea layer depends on coordinator projections, not direct device backends.
- Persisted formats stay owned by `internal/mapfile`, `internal/catalog`, and the documents under `docs/formats/`.
- Cross-platform development support stays in `internal/platform` so Windows-local simulator and TUI work do not weaken Linux production boundaries.

## Verification

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`

Target-native package compile evidence is also required for platform changes:

- `GOOS=linux GOARCH=amd64 go test -c ./internal/platform`
- `GOOS=darwin GOARCH=amd64 go test -c ./internal/platform`
- `GOOS=windows GOARCH=amd64 go test -c ./internal/platform`

## Audited cohesive modules

The following modules were reviewed during the SRP refactor and remain
intentionally cohesive because their responsibilities share one change reason
or are already narrow effect boundaries:

- `cmd/discrescue` remains the executable bootstrap and worker-mode entrypoint.
- `internal/buildinfo` remains embedded build metadata only.
- `internal/image` keeps positioned writes with their commit ordering because
  durability and image-position correctness must change together.
- `internal/logging` keeps bounded event construction and redaction together at
  the logging boundary.
- `internal/device/linux` keeps the native block/`SG_IO` adapter and its CDB
  encoding together because they form one platform mechanism boundary.
- Recovery pass implementations remain cohesive state machines; scheduling
  policy is separated from lifecycle and execution boundaries without splitting
  a single pass across artificial files.
