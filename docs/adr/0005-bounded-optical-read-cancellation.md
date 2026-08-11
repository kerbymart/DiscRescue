# ADR 0005: Bounded optical reads with reopenable cancellation

## Status

Accepted

## Date

2026-08-10

## Context

Optical-drive reads can spend an unbounded amount of time in the operating
system or drive firmware retrying a damaged range. A synchronous `ReadAt`
therefore prevented the scheduler from recording a deferred range and moving
to later LBAs. The existing `ReadDeadlinePolicy` was present but unused.

## Decision

Recovery policies define positive healthy and damaged soft/hard deadlines.
The scheduler applies the healthy hard deadline during fast acquisition and
the damaged hard deadline during trim, adaptive, and targeted passes.

Native adapters wrap their raw source in `recovery.ReopenableReaderAt`. On a
read deadline it closes the active source, waits for the request to finish,
reopens the same source, and only then returns the timeout to the scheduler.
On a user stop or force-stop cancellation it closes and joins the active
request without reopening, because the job is terminating. This preserves
serialized device access and makes later LBAs safe to read. A source that
cannot implement this cancellation boundary must not be used as a native
optical adapter.

## Consequences

- A timed-out damaged range remains retryable/deferred and does not become
  permanently unreadable solely because it was slow.
- Later readable regions can continue through the fast pass.
- Stop and force-stop can interrupt the same serialized source boundary
  without leaving a blocked read running concurrently with a new request.
- Reopening a raw device can still fail; that is reported as a fatal source
  failure rather than hidden as a media error.

## Verification

- Deterministic reopenable-reader tests prove cancellation, joining, and
  reopening.
- Recovery strategy tests prove a timed-out fast read is deferred and later
  LBAs are recovered.
- Run `go test ./...`, `go test -race ./internal/recovery ./internal/platform`,
  `go vet ./...`, and Darwin/Linux/Windows cross-builds.
