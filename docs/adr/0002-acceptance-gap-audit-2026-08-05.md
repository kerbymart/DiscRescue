# ADR 0002: Accepted Acceptance Gaps for the Windows Simulator Phase

## Status

Accepted

## Date

2026-08-05

## Context

An acceptance audit was run on Wednesday, August 5, 2026 against the current repository state using TDD Sections 22, 23, and 27 as the verification boundary. The codebase now has broad simulator, TUI, verification, merge, and release-gate coverage, but some acceptance items still require later Linux hardware or long-run release validation before they can be claimed as fully satisfied.

## TDD Requirements

- Section 22.2: simulator-backed recovery work is the primary development boundary.
- Section 22.5: Linux integration matrix covers hardware, filesystem, permission, and multi-drive scenarios.
- Section 22.6: soak testing includes 24-hour runs, repeated restarts, attach/detach, and leak monitoring.
- Section 23: functional, reliability, and performance acceptance criteria define the conformance target.
- Section 27: the requirements coverage matrix ties capabilities to their principal verification method.
- Section 24: release properties and platform limitations must be documented.

## Decision

The repository will accept the following gaps as phase-appropriate on Wednesday, August 5, 2026:

- Linux hardware integration coverage is not yet present in repository evidence.
- Long-run soak evidence is still narrower than the full TDD scope.
- Race coverage is part of the release gate, but it is still skipped on the current Windows environment unless `CGO_ENABLED=1`.
- Performance acceptance targets are only partially covered by current benchmark commands because the repository does not yet encode the final target thresholds or a healthy-device benchmark fixture.
- Some acceptance items remain partially covered because they are implemented as bounded unit, simulator, or TUI slices but do not yet have a named end-to-end acceptance scenario artifact.

These gaps are accepted only as a current-phase audit outcome. They do not reduce the eventual v1.0 acceptance scope.

## Alternatives Considered

- Claim current simulator and unit coverage as equivalent to the full Linux integration matrix.
- Delay the acceptance audit until all Linux hardware and long-run soak evidence exists.
- Record the gaps informally outside the ADR set.

## Consequences

- The repository now has an explicit written acceptance audit tied to current evidence.
- The project cannot yet claim full Section 23 acceptance closure or Milestone 6 completion solely from the current Windows-local evidence.
- Future Linux packaging, hardware matrix, race coverage, and long-run soak work must close these accepted gaps rather than treating them as permanently waived.

## Migration or Rollback

- Update or supersede this ADR when Linux integration, long-run soak evidence, race coverage, and final performance thresholds become part of the repository evidence.
- If later evidence closes all listed gaps, mark this ADR as superseded by the newer acceptance audit record.

## Verification

- `go test ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/discrescue`
- `powershell -ExecutionPolicy Bypass -File scripts/release-gates.ps1`
- `docs/architecture/acceptance-coverage.md`
