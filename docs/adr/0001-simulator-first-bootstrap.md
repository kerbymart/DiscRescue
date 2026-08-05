# ADR 0001: Simulator-First Bootstrap Boundary

## Status

Accepted

## Date

2026-08-05

## Context

The TDD defines Linux optical-device access as the product target, while current development is happening on Windows. The repository needs an initial implementation boundary that allows progress on state transitions, TUI behavior, formats, and test infrastructure without inventing device behavior or weakening read-only guarantees.

## TDD Requirements

- Section 2.3.13: source media access must remain read-only.
- Section 2.3.14: blocking device I/O must stay outside the Bubble Tea event loop.
- Section 6.6: device I/O is isolated from the TUI.
- Section 14: package ownership must follow the documented internal layout.
- Sections 22 and 23: simulator coverage is the primary verification boundary.

## Decision

The bootstrap repository state will treat simulator-backed behavior, pure transitions, and Bubble Tea shell work as the first implementation boundary.

Specifically:

- Windows is used for local development, formatting, unit tests, vet, build checks, and TUI iteration.
- Linux optical-device adapters remain placeholders until their format contracts, command boundaries, and tests are ready.
- The executable remains single-binary, with a worker mode switch already reserved in `cmd/discrescue`.
- New implementation work should prefer deterministic transition logic and simulator scenarios before real hardware integration.

## Alternatives Considered

- Implement Linux optical-device access immediately on Windows through compatibility layers.
- Delay all code until every owned format document is complete.
- Build imperative shell scripts around device probing before defining package boundaries.

## Consequences

- The current codebase can compile and be verified on Windows without claiming hardware completeness.
- Later Linux device work must replace placeholders with public-spec-based implementations and tests.
- Format and protocol tasks remain critical next steps because bootstrap stubs are not stability guarantees.

## Migration or Rollback

- Replace placeholder Linux adapter types with real implementations once their related tasks are done.
- Update this ADR if the bootstrap boundary changes from simulator-first to hardware-first for a later milestone.

## Verification

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`
