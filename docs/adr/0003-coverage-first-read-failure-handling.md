# ADR 0003: Coverage-first read-failure handling

## Status

Accepted

## Date

2026-08-10

## Context

The recovery scheduler previously aborted a job after a fixed number of
consecutive failed reads. A damaged contiguous region could therefore leave
later LBAs unvisited even while the source remained addressable. Issue #79
requires recovery completion to be defined by durable coverage, not by an
arbitrary count of media-read failures.

## TDD Requirements

- Epic 3 Sections 4.4, 8, and 9: policy-driven fast, trim, adaptive, and
  targeted passes with bounded per-range attempts.
- Epic 3 Sections 58 and 59: Fast/Gentle may finish with deferred work;
  only `FinalizeUnresolved=true` promotes exhausted ranges to missing.
- `AGENTS.md` invariants: preserve read-only operation, durable map ordering,
  bounded retries, truthful confidence, and resumability.

## Decision

Media read failures are durable `io_error` retryable extents and do not abort
the job solely because they are consecutive. The fast pass continues through
the known capacity, then configured retry passes operate on deferred extents.
Balanced finalization marks remaining retryable ranges `missing`.

Permission, missing-source, and platform-confirmed disconnected-device errors
remain fatal. Native read errors are retained in bounded progress diagnostics.

## Alternatives Considered

- Retain a larger consecutive-failure threshold: still leaves known LBAs
  uncovered and cannot distinguish media damage from source loss.
- Treat every read error as fatal: prevents recovery from damaged media.
- Treat every read error as non-fatal: can conceal confirmed source loss or
  revoked access.

## Consequences

- Completed recovery can truthfully contain unreadable sectors.
- Fast and Gentle can complete their configured passes with deferred sectors.
- The recovery UI can display durable overall coverage separately from bytes
  successfully recovered.
- Platform adapters classify only clearly fatal source failures; ambiguous
  media I/O remains retryable to protect damaged-disc recovery.

## Migration or Rollback

No persisted format version changes. Existing `unknown`, `queued`, and
`io_error` extents remain retryable. A future source-specific classifier can
add more confirmed fatal errors without changing map semantics.

## Verification

- `internal/platform/recovery_strategy_test.go` covers damaged bands, an
  entirely unreadable medium, diagnostics, source-access failure, and final
  coverage invariants.
- `internal/app/view_test.go` covers the visual coverage rail.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
  `go build -trimpath ./cmd/discrescue`.
