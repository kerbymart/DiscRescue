# ADR 0004: User-initiated unresolved retry cycles

## Status

Accepted

## Date

2026-08-10

## Context

Fast and Gentle recovery may complete with deferred ranges. Balanced recovery
may complete with ranges marked missing after its bounded retry budget is
exhausted. The recovery summary needs a direct retry action without claiming
that recovered sectors can be safely overwritten or that retries are
unbounded.

## TDD Requirements

- TUI redesign plan Section 7.7: deferred and unreadable counts remain
  distinct, and retrying deferred sectors is the safe default.
- ADR 0003: media read failures remain durable retryable state and every retry
  has an explicit finite bound.
- `AGENTS.md`: the map is authoritative, device access remains read-only, and
  resumability and durable ordering take precedence over throughput.

## Decision

The summary's primary action starts a new Balanced recovery cycle using the
existing image and `.drmap`. Only unresolved extents are revisited; recovered
extents remain untouched. The map's `attempts` field remains cumulative.

For this explicit retry cycle, scheduler pass limits are offset by the highest
current attempt count among unresolved extents. This gives each unresolved
extent one fresh, finite policy budget while preserving durable attempt
history. The rule is implemented in the shared platform recovery strategy and
therefore applies equally to macOS, Windows, and Linux adapters.

## Consequences

- Summary states with deferred, unreadable, or both kinds of unresolved work
  offer a clearly labelled retry action as the keyboard-default choice.
- A retry never writes to recovered image extents and uses the same validated
  image/map pair.
- Repeated user retries remain finite per action; the map records cumulative
  attempts for diagnostics and later inspection.

## Verification

- `internal/app` tests cover segmented state presentation and the summary
  retry transition.
- `internal/platform` tests cover retry-budget derivation from persisted
  attempt counts.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and native
  builds for supported platforms.
