# DiscRescue contributor instructions

## Scope and authority

This file applies to the entire repository. `.project/discrescue-tdd.md` is the normative product and architecture contract. Read the sections relevant to a change before editing, and treat its **MUST**, **MUST NOT**, **SHOULD**, and **SHOULD NOT** language literally. If this file and the TDD appear to disagree, follow the TDD and update this file in the same change.

DiscRescue is a Linux-first, Go 1.25+ optical-disc recovery application. It ships as one `discrescue` executable with a Bubble Tea v2 TUI and an internal self-executed device-worker mode. The initial implementation boundary is CD/DVD recovery; do not expand it into general block-device recovery, filesystem repair, decryption, or media writing.

Use only public Go, OS, SCSI, and MMC specifications when implementing device behavior. Do not copy another recovery application's source, control flow, private formats, internal names, comments, or diagnostic text. Project-owned persisted formats and protocols must be independently documented and versioned.

## Working agreement

Before making a change:

1. Read TDD Sections 2.3 and 6, then the component, testing, and acceptance sections affected by the task.
2. Inspect the current implementation, tests, format documents, and ADRs. The TDD describes the destination; the repository describes the implemented state.
3. State the behavior and invariant being changed. Identify the authoritative state owner, external effects, failure paths, and verification command.
4. Prefer the smallest milestone-aligned vertical slice that can be tested through the simulator. Preserve public APIs, persisted formats, and user-visible behavior unless the task explicitly changes them.
5. Add or update a failing test before implementation when behavior is new or defective. For a behavior-preserving refactor, establish characterization coverage first.

Do not silently settle an item in TDD Section 26 when it affects safety, identity matching, confidence, a persisted format, or a user-visible guarantee. Record the decision in `docs/adr/`, reference the applicable TDD requirement, and update format documentation and tests with the code. Small reversible implementation details may use a documented assumption.

## Non-negotiable invariants

- Source media is read-only. Production code may issue only the enumerated non-destructive commands. Never add burn, blank, format, close-session, reserve-track, controller-reset, arbitrary-shell, or automatic-unmount behavior.
- Preserve `image offset = LBA * logical sector size`, including unreadable sectors. Write complete sectors only.
- The `.drmap` state is authoritative; placeholder or zero image bytes never prove recovery.
- Persist image data successfully before committing a map transition that claims the data exists. UI progress reflects durable state, not merely completed device I/O.
- Every read, retry, wait, queue, pass, split, speed change, reopen, reset, worker restart, and shutdown step has an explicit finite bound and deterministic termination condition.
- Potentially blocking device work runs in the worker subprocess, never in Bubble Tea `Update` or `View`. Allow at most one active device command and one worker owner per drive.
- The coordinator is the sole authority for job transitions and the in-memory sector/extent index. Other goroutines receive immutable snapshots and submit commands or results.
- Pause, cancellation, process failure, ENOSPC, and worker failure preserve a resumable image/map pair whenever one was validly established.
- Resume and automatic merge require strong logical-content identity. Any geometry, layout, or overlapping-sample conflict blocks automatic mutation of an existing job.
- Logical-content, capture, device, and optional physical-copy identities remain separate. Drive identity and user labels never contribute to content matching.
- Confidence is explicit. A successful read, repeated agreement, cross-capture agreement, trusted checksum, reconstruction, tentative data, and conflict are not interchangeable.
- Catalog history is local, bounded, crash-safe, and non-authoritative for recovery. Catalog failure or lock contention warns but does not abort a safe recovery.
- No unbounded goroutines, channels, read payloads, buffers, in-memory logs, journal growth, or retained snapshots.
- Logs do not write to stdout while the TUI is active. Terminal state is restored after normal exit, cancellation, signals, and handled fatal errors.

When safety and throughput conflict, preserve media read-only operation, correct sector placement, durable resumability, and honest confidence reporting before optimizing speed.

## Architecture and ownership

Keep dependencies directed through project-owned boundaries:

```text
Bubble Tea UI -> coordinator -> recovery schedulers
                           -> image writer
                           -> map/journal
                           -> catalog
                           -> worker supervisor <-> device worker <-> device backend
```

Follow the package responsibilities in TDD Section 14:

- `internal/app`: TUI model, messages, update, view, keys, and presentation-only projections.
- `internal/coordinator`: job orchestration, authoritative transitions, cancellation, checkpointing, and shutdown.
- `internal/recovery`: deterministic fast, trim, adaptive, scrape, and verification scheduling.
- `internal/device`: bounded protocol, supervisor, worker, media model, and sense classification.
- `internal/device/linux`: Linux block and `SG_IO` adapters and MMC/CDB encoding.
- `internal/image`: positioned sector writes and ISO/BIN/CUE representation.
- `internal/mapfile`: `.drmap` format, journal, checkpoints, extents, and replay.
- `internal/catalog`: content identity, lookup, local journal, snapshot, and compaction.
- `internal/integrity` and `internal/merge`: verification, reconstruction boundary, consensus, and provenance.
- `internal/logging`: bounded structured events and redaction.
- `internal/testdevice`: deterministic device simulation and fault scripts.

Keep packages internal before v1.0 unless a genuine public API requirement is approved. Define interfaces at the consumer boundary and keep them narrow; do not create interface layers without a real alternate implementation, effect boundary, or test seam. The worker never owns image, map, catalog, or UI state. The UI never talks directly to device backends.

## State and effect design

Apply The Elm Architecture to interactive and job-control flows:

- `Model` contains the complete state required to choose the next transition and render the current view. Recompute cheap derived values.
- Messages describe events or intent, such as `SaveRequested` or `ReadCompleted`; avoid field-setter messages such as `SetSavingTrue`.
- `Update(message, model)` is deterministic and returns the next model plus effect descriptions. It performs no device I/O, filesystem access, hashing, sleeping, subprocess waiting, clock reads, or blocking channel receives.
- `View(model)` is a pure projection. Event handlers emit messages and never mutate state directly.
- Every asynchronous effect reports success or failure through a typed message. Use request or generation IDs when a stale result could overwrite newer intent.
- Represent mutually exclusive workflow states with explicit enums or tagged structures rather than contradictory boolean clusters.
- Test transitions as `(model, message) -> (next model, effects)`, including invalid, duplicate, stale, cancellation, and failure sequences.

Use data-oriented design where it fits Go and this domain:

- Model sector extents, requests, results, snapshots, protocol frames, journal records, catalog events, and configuration as explicit data.
- Prefer pure functions for scheduling, identity encoding/comparison, extent transformations, confidence selection, and transition calculation.
- Separate mutation into **calculate -> validate -> commit -> publish effects**. Never perform external I/O in a calculation that may be retried.
- Treat published snapshots as immutable. Copy or replace owned data rather than mutating data visible to another goroutine.
- Keep schemas and validation rules separate from binary/JSON/TOML representations. Validate all untrusted boundaries before allocation or state transition.
- Use typed structs and domain enums for stable Go contracts; do not replace them mechanically with `map[string]any`. Use maps for genuine indexes or open data and document their key vocabulary.
- Compare structured data structurally, not by serialized string. Capture reproducible simulator inputs and relevant nondeterministic values without secrets or raw user data.

## Code quality rules

- Optimize in this order: correctness, safety, clarity, testability, then measured performance.
- Use intent-revealing domain names and the TDD's project terminology. Keep functions focused at one abstraction level and make side effects obvious at call sites.
- Prefer explicit control flow over clever generic machinery. Extract repeated business meaning, not coincidental syntax.
- Preserve error context. Errors should identify the operation and, when applicable, device, LBA/count, retryability, sense tuple, OS error, worker request ID, and map sequence. Do not log and swallow errors.
- Wrap OS, filesystem, terminal, and third-party APIs behind small project-owned adapters. Validate protocol lengths before allocating.
- Comments explain intent, invariants, public contracts, public-spec references, unusual ordering, and safety constraints. Do not narrate obvious code or leave commented-out code.
- Avoid mutable globals, hidden clocks/randomness, flag arguments, panic as routine control flow, and background goroutines without a clear owner.
- A goroutine must have an owner, context, bounded input, error path, and shutdown position. Keep device concurrency serialized.
- Use the standard library unless a dependency materially reduces risk or implements a required public API. Bubble Tea v2 must use `charm.land/bubbletea/v2`.
- Measure before adding complexity to the healthy-media path. Retain extent-based/coalesced state changes and backpressure; never trade away crash ordering for throughput.

## Product and TUI rules

The target user behavior is: select a drive, understand whether matching contents were processed before, choose a safe action, and complete or stop a recoverable job without needing optical-recovery expertise.

- Present a guided, shallow, one-column flow with one primary decision at a time, one highlighted choice, and at most one modal.
- Keep advanced settings and technical details secondary. The default recovery page has one progress bar and no heat map, live log, chart, dense table, SCSI fields, or decorative telemetry.
- Use plain, action-oriented language from TDD Section 19.12. Say **matching contents**, never **same physical disc**, unless a user supplied a physical identifier.
- Treat identity and ETA claims as hypotheses constrained by evidence. Conflicts defeat a match; insufficient samples are indeterminate; low-confidence ETA is hidden.
- Choose user-protective defaults: verify a strong completed match, resume a strong incomplete match, save progress before stopping, and place immediate worker termination last.
- Never use a success treatment when sectors are missing, conflicting, checksum-invalid, or accepted only under an explicit unverified policy.
- Explain whether work remains resumable and give the next safe action after every recoverable error.
- Keep context-specific keyboard hints visible. All functions must work by keyboard; color may reinforce but never carry meaning alone.
- Support the TDD's 80x24, 60x18, and 40x12 layouts, a resize-only view below minimum size, monochrome output, Unicode/path wrapping, and resize without job interruption.
- Treat UX choices as testable assumptions. Cover wording, safe defaults, comprehension-critical states, and guardrails with golden views and keyboard-flow tests.

Do not optimize engagement or urgency. Optimize for a correct archive, informed control, truthful status, successful resume, and avoidance of destructive mistakes.

## Persistence, protocol, and compatibility

Every project-owned persisted format or IPC protocol must have a versioned specification under `docs/formats/`. The specification and implementation change together and define:

- magic/version and byte order;
- field sizes and canonical encodings;
- maximum record and payload lengths;
- CRC/integrity behavior;
- unknown record/version handling;
- compatibility and migration policy;
- commit, flush, compaction, and crash-recovery semantics;
- truncation, corruption, deletion, and partial-write behavior.

Parsers must reject overflow, impossible lengths, invalid ranges, overlapping extents, and incompatible identity before allocation or mutation. Replay accepts a truncated final journal record only as specified; it must not reinterpret corrupt committed history. Preserve older readers or provide an explicit migration when changing a frozen version.

The worker protocol uses bounded, length-prefixed frames with monotonically increasing request IDs and CRC32C. Do not place unbounded sector data, logs, or paths under worker control. Catalog and map journals are independent failure domains.

## Test-driven development and verification

Use the deterministic device simulator as the default development boundary. Real optical hardware complements simulator coverage; it does not replace it.

For each change:

1. Derive a concrete scenario from TDD Sections 22, 23, or 27.
2. Write the smallest failing unit, transition, simulator, golden, or fault-injection test.
3. Implement only enough behavior to pass while preserving the invariants above.
4. Refactor under green tests for clear names, small boundaries, and deterministic logic.
5. Run the narrow affected tests, then the repository-wide checks appropriate to the risk.

Required test emphasis:

- fixed byte vectors for CDBs, MMC structures, frames, CRCs, and sense parsing;
- table-driven extent, scheduler, identity, confidence, error, and retry-budget tests;
- replay at every truncation/kill point for journals and checkpoints;
- structural equality tests for pure transformations and snapshots;
- simulator scenarios for failed ranges, partial reads, delays, conflicting capacity, media replacement, worker crash/hang, corruption, and ENOSPC;
- TUI transition and golden tests across supported sizes, colors, long text, safe modal defaults, stale messages, and final unthrottled state;
- integration tests for resume without rereading verified sectors, wrong-media rejection before mutation, catalog independence, conservative merge, and command audit;
- race, leak, soak, throughput, and CPU checks before release or when concurrency/performance changes.

Once the Go module and packages exist, the baseline commands are:

```text
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
go build -trimpath ./cmd/discrescue
```

Run focused package tests during iteration. Run all tests for cross-package state, protocol, persistence, concurrency, or release changes. A static `CGO_ENABLED=0` release build is preferred only where it preserves device access and terminal restoration. Report commands not run and the reason.

## Documentation

- Keep architecture under `docs/architecture/`, decisions under `docs/adr/`, and owned formats under `docs/formats/` as required by TDD Section 28.1.
- Update documentation in the same change as an interface, invariant, command set, persisted field, state transition, default, or user-visible workflow.
- ADRs record context, TDD requirements, alternatives, decision, consequences, migration/rollback, and verification.
- Exported Go declarations receive concise contract documentation. Internal comments focus on non-obvious intent and safety constraints.
- User documentation uses the primary-screen vocabulary, distinguishes completed/verified/incomplete/resumable states, and never overclaims physical identity or recovery confidence.
- Examples must use synthetic paths, identifiers, sector data, and device metadata. Do not commit real disc contents, personal labels, secrets, or sensitive logs.

## Completion checklist

A change is complete only when all applicable items are true:

- The behavior maps to a TDD requirement, milestone, or approved ADR.
- Authoritative state has one owner; effects and retry boundaries are explicit.
- Device access remains read-only and every new loop, queue, payload, timeout, and retry is bounded.
- Data-before-map ordering, resume identity checks, and catalog independence are preserved.
- Normal, failure, cancellation, stale-result, and crash/replay paths are tested at the appropriate layer.
- TUI copy is truthful, the safe action is the default, keyboard/monochrome/small-terminal behavior remains usable, and technical detail stays secondary.
- Format/protocol and architecture documentation matches the code.
- Changed Go files are formatted; relevant tests, race checks, vet, build, and command-audit checks pass or omissions are reported.
- The final diff contains no unrelated rewrites, generated debris, sensitive data, or accidental changes to user work.
