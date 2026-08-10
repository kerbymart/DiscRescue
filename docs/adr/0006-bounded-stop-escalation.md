# ADR 0006: Bounded stop escalation for active device requests

## Status

Accepted

## Date

2026-08-10

## Context

Pause and save-progress-and-stop must stop scheduling new reads immediately,
but the currently active optical request may still be inside the operating
system or drive firmware. Previously the UI exposed a five-second escalation
state while describing the implementation as a worker, even though the
relevant safety boundary is the active device request.

## Decision

Use a shared five-second `DefaultStopGracePeriod` for native adapters. A stop
request closes the scheduling gate and allows the current request to finish
cooperatively. If it remains active after the grace period, the UI presents an
explicit force-stop action. The action closes the serialized reopenable source
boundary, joins the active read, and lets the existing durable checkpoint and
device-release path complete. `x` and `Ctrl+C` share this explicit escalation
binding.

The UI and status messages call this the active device request. They do not
claim that a separate worker process is being terminated.

## Consequences

- No new optical read starts after pause or stop is requested.
- A normal stop completes when the bounded read deadline returns and the
  durable checkpoint succeeds.
- A blocked request has a deterministic escalation state and a force-stop
  path that uses the same serialized source cancellation boundary.
- Force stop remains exceptional and is reported as preserving the last valid
  durable image/map state.

## Verification

- Lifecycle and stop-controller tests cover the scheduling gate, grace
  escalation, force-stop, checkpoint, and release ordering.
- Reopenable-reader tests cover interrupting an active request without leaving
  a concurrent source access.
- TUI tests cover force-stop availability and the active-device-request copy.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and native
  cross-builds.
