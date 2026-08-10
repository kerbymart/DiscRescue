# DiscRescue contributor instructions

## Scope and source of truth

This file applies to the entire repository.

DiscRescue is a Go 1.26.5+ optical-disc recovery application for Linux, macOS, and Windows. It ships as one `discrescue` executable with a Bubble Tea v2 terminal UI. Production optical-device access is platform-specific and read-only. The product boundary is CD/DVD optical recovery; do not expand DiscRescue into general block-device recovery, filesystem repair, decryption, or media writing unless an approved issue and architectural decision explicitly change that scope.

Use the following source-of-truth order when planning or reviewing a change:

1. **Current source code and tests** describe behavior that is actually implemented.
2. `README.md` describes the supported toolchain, product overview, and top-level workflows.
3. `docs/architecture/` records current architecture, platform, validation, and development contracts. In particular, consult `docs/architecture/cross-platform-development.md`, `docs/architecture/validation-workflow.md`, and `docs/architecture/native-platform-parity.md` when they apply.
4. `docs/adr/` records accepted architectural and safety decisions.
5. `docs/formats/` defines project-owned persisted formats and protocols.
6. The active GitHub issue or EPIC defines the scope and acceptance criteria for the work being performed.

Historical planning documents are not authoritative merely because an older document or comment references them. If a repository document conflicts with current source/tests or references a path that no longer exists, treat that as documentation drift: do not silently invent the missing contract. Reconcile the document in the same change when it is in scope, or file/link a focused follow-up documentation issue.

Use only public Go, operating-system, SCSI, MMC, filesystem, and terminal specifications when implementing device behavior. Do not copy another recovery application's source, control flow, private formats, internal names, comments, or diagnostic text. Project-owned persisted formats and protocols must be independently documented and versioned.

## Supported development contract

- Use Go 1.26.5 or newer.
- Support Linux, macOS, and Windows development.
- Keep production source-media access read-only on every platform.
- Preserve the pure-Go, cross-platform build goal. Do not introduce a required shell utility, shell-specific task runner, CGO dependency, Xcode/SDK dependency, or host command into a native device path unless the issue explicitly requires it and the tradeoff is approved/documented.
- Keep platform-specific discovery, raw-device access, media inspection, and eject mechanics behind project-owned boundaries. Shared recovery policy, durability, lifecycle, and UI semantics remain platform-neutral.
- Use project-owned capability and error types instead of leaking native handles or OS-specific types into application code.
- Consult `docs/architecture/native-platform-parity.md` and target-OS development notes for operation-level capability details.
- Treat cross-compilation as supplemental evidence only. Claims about real optical-drive behavior require validation on representative target hardware.

## Working agreement

Before making a change:

1. Read the active issue and its acceptance criteria.
2. Read the relevant `README.md`, architecture documents, ADRs, and format specifications.
3. Inspect the current implementation and tests in the affected packages. Repository state is more authoritative than historical plans.
4. State the behavior and invariant being changed. Identify the authoritative state owner, external effects, failure paths, cancellation/timeout boundaries, persistence impact, and verification commands.
5. Prefer the smallest issue-scoped vertical slice that can be verified deterministically.
6. Add or update a failing test before implementing new or defective behavior when practical. For behavior-preserving refactors, establish characterization coverage first.
7. Preserve public APIs, persisted formats, recovery semantics, and user-visible behavior unless the issue explicitly changes them.

When a change decides or alters a safety guarantee, content-identity rule, recovery confidence rule, persisted format, protocol, platform capability contract, or user-visible guarantee, record the decision in `docs/adr/` when the rationale is not already captured. Update affected architecture/format documentation and tests in the same change.

## Engineering judgment and uncertainty

Repository rules are defaults for producing a safe, understandable system; they are not a substitute for technical judgment. Non-negotiable recovery/safety invariants remain absolute unless an approved architectural decision explicitly changes the product contract. For other design choices, evaluate the current context instead of applying patterns mechanically.

Before making a non-trivial architectural decision, identify:

- the user journey or concrete problem being improved;
- the current issue scope, delivery horizon, and expected lifetime of the decision;
- the team's ability to understand, test, and maintain the design;
- whether the path is performance-sensitive or latency-bound;
- which parts are easy to change later and which contracts will be expensive to change;
- the hard problems, unknown OS/device behaviors, and assumptions that could invalidate the design;
- alternatives, trade-offs, failure modes, and the evidence supporting the selected option.

Use these decision rules:

- Prefer an iterative thin vertical slice that works end to end over building disconnected layers that integrate late. Integrate small changes early and learn from executable feedback.
- Make reversible decisions as late as practical when delay allows better evidence. Do not delay hard problems whose uncertainty can invalidate the architecture.
- Design deeply the things that are expensive to change - persisted formats, protocols, content identity, durable state semantics, project-wide interfaces, platform capability contracts, and established user-visible vocabulary - but implement only the portion required by the current issue.
- Resolve important unknowns with the smallest bounded experiment, proof of concept, simulator scenario, or hardware probe that can answer the question. Record what the evidence proves and what it does not prove.
- Do not debate a measurable claim indefinitely. When a safe, short experiment can answer it, measure it.
- Optimize from evidence. A performance change should identify the workload/bottleneck it addresses and, when practical, record a comparable baseline and post-change measurement.
- Avoid speculative flexibility, speculative scale, and speculative generalization. Architectural options have maintenance and cognitive costs.
- Prefer the simplest design the current team can operate correctly. Complexity that requires rare expertise must be justified by a concrete requirement or measured limitation.
- If an exception to a repository-wide design principle establishes a lasting precedent, record the rationale in the PR and an ADR when appropriate.

## Universal design principles

Apply the following principles across production code, tests, tools, and architecture as repository-wide defaults. They guide implementation and review, but they do not override stronger DiscRescue safety, correctness, durability, or compatibility contracts.

### SRP  -  Single Responsibility Principle

A package, type, function, component, or module should have one coherent responsibility and one primary reason to change.

- Keep policy separate from mechanism.
- Keep UI state/presentation separate from device, filesystem, process, and persistence effects.
- Keep platform-neutral recovery behavior separate from OS-specific discovery, raw-device, ioctl, and eject mechanics.
- Keep persistence encoding/replay separate from recovery scheduling decisions.
- Split code when unrelated responsibilities evolve for different reasons.
- Do not split cohesive behavior merely to make functions/files artificially small.

### DRY  -  Don't Repeat Yourself

Maintain one authoritative representation of each business rule, invariant, algorithm, state transition, format rule, protocol behavior, and shared vocabulary.

- Extract duplicated business meaning, not merely similar-looking syntax.
- Prefer one source of truth for recovery states, retry policy, capability interpretation, error classification, key bindings/help, persisted-format rules, and other durable contracts.
- Derive presentation/summary data from authoritative state instead of maintaining independently mutable copies.
- Do not create an abstraction merely because two short pieces of code currently look alike.
- Limited duplication is preferable when eliminating it would create artificial coupling, hide intent, or force unrelated components to evolve together.

### KISS  -  Keep It Simple

Prefer the simplest design that correctly satisfies the current contract.

- Prefer explicit Go control flow and domain types over clever generic machinery.
- Prefer standard library and established project mechanisms before adding frameworks or custom infrastructure.
- Minimize layers, indirection, concurrency, configuration, dependencies, and hidden state.
- When two designs satisfy the same requirements, prefer the one requiring less non-local context and fewer states to reason about.
- Complexity requires a concrete benefit that outweighs its correctness, cognitive, testing, and maintenance cost.

### YAGNI  -  You Aren't Gonna Need It

Do not implement speculative capabilities.

- Do not add extension points, configuration knobs, generalized frameworks, hypothetical platform abstractions, future media formats, or recovery modes without a current requirement.
- Do not design for hypothetical scale/performance without evidence that the current design is insufficient.
- Preserve an obvious future option when doing so is nearly free, but do not pay present complexity for an unproven future need.
- Delete obsolete code/abstractions rather than retaining them for a possible future use.

### SOLID  -  apply where it fits Go

Use SOLID as a design-review framework, adapted to Go rather than copied from inheritance-heavy languages:

- **S  -  Single Responsibility:** keep responsibilities cohesive and ownership explicit.
- **O  -  Open/Closed:** prefer stable, narrow boundaries when adding a real new implementation is simpler/safer than repeatedly editing established contracts; do not create speculative extension systems.
- **L  -  Liskov Substitution:** every implementation of an interface must preserve the behavioral contract expected by its consumer, including error, cancellation, ownership, and durability semantics.
- **I  -  Interface Segregation:** define narrow interfaces at the consumer boundary; callers should not depend on operations they do not use.
- **D  -  Dependency Inversion:** platform-neutral recovery and application policy should depend on project-owned abstractions rather than native OS handles, filesystem details, terminal internals, or third-party implementation types.

Do not mechanically introduce interfaces, factories, adapters, generic layers, or embedding merely to claim SOLID compliance. In Go, prefer concrete types until a real effect boundary, alternate implementation, ownership seam, platform seam, or test seam exists.

### Separation of concerns

Keep presentation, policy, orchestration, native mechanism, persistence, identity, verification, and diagnostics separate enough that changing one does not require unrelated knowledge or mutation. Avoid modules that mix user interaction, recovery decisions, raw I/O, and durable state changes in one control path.

### Principle of least astonishment

Names, APIs, return values, state transitions, and UI actions should behave as a competent Go/DiscRescue contributor would reasonably expect. Do not use familiar names for surprising semantics. Make unusual behavior explicit in the type/name/contract rather than relying on hidden comments or tribal knowledge.

### Make invalid states hard to represent

- Prefer domain enums/types and constructors/validation over magic strings, sentinel numbers, or contradictory booleans.
- Make required inputs required. Do not silently convert a missing critical input into a no-op or unrelated default.
- Enforce necessary assumptions at the boundary rather than expecting callers to remember them.
- Prefer immutable snapshots/copies across ownership boundaries. Do not unexpectedly mutate caller-owned inputs.
- If callers must remember a hidden call order, temporal coupling, or undocumented precondition to remain safe, redesign the API when practical.

### Principle precedence

When these principles conflict, preserve the stronger DiscRescue contract in this order:

1. source-media safety and non-destructive behavior;
2. recovery correctness and exact sector placement;
3. durable resumability and persisted-format compatibility;
4. bounded device ownership, cancellation, and termination;
5. truthful user-visible state, identity, confidence, and errors;
6. simplicity and cognitive clarity;
7. SRP / DRY / KISS / YAGNI / SOLID and related design principles;
8. measured performance optimization.

An intentional departure from a design principle should make the reason visible in code/tests and the PR; use an ADR when the exception becomes a lasting architectural precedent.

## Issue, branch, and commit workflow

Prefer one issue and one coherent change set per branch.

Use issue-specific branch names with a short lowercase slug:

```text
feat/issue-<N>-<slug>
fix/issue-<N>-<slug>
docs/issue-<N>-<slug>
test/issue-<N>-<slug>
refactor/issue-<N>-<slug>
chore/issue-<N>-<slug>
```

Use another precise Conventional Commit type as the branch prefix when appropriate, such as `build`, `ci`, or `perf`.

Only stack one issue branch on another when there is an explicit dependency. Record that dependency in the issue or pull request and make the intended base branch clear. Otherwise branch from the current default branch and keep unrelated work separate.

Every commit, including a squash or merge commit created for a pull request, MUST follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```text
<type>[optional scope][optional !]: <description>

[optional body]

[optional footer(s)]
```

- Use `feat` for a new user-facing capability and `fix` for a defect correction.
- Use `docs` when documentation is the primary change, including contributor guidance and issue templates.
- Use `test` for test-only changes, `refactor` for behavior-preserving restructuring, and `chore` for repository maintenance that does not fit a more specific type.
- Precise types such as `build`, `ci`, `perf`, `revert`, and `style` may be used when appropriate.
- Write the description as a concise imperative summary.
- An optional scope should be a noun naming the affected area, such as `tui`, `device`, `recovery`, or `mapfile`.
- Mark a breaking change with `!` before the colon or a `BREAKING CHANGE: <description>` footer only when a supported contract truly changes.
- Keep each commit focused on one coherent change. Split mixed-purpose commits when practical.

Stage and commit only intended files. Never absorb unrelated local work merely because it is present in the working tree.

## Non-negotiable recovery and safety invariants

- **Source media is read-only.** Production code may issue only approved non-destructive operations. Do not add burn, blank, format, close-session, reserve-track, arbitrary write, controller-reset, arbitrary-shell, or automatic-unmount behavior. Eject is an explicit user-initiated device operation, not an implicit recovery side effect.
- Preserve `image offset = LBA * logical sector size`. Write complete logical sectors only and preserve the logical position of unreadable sectors.
- The `.drmap` state is authoritative for recovery state. Placeholder, sparse, or zero-filled image bytes do not prove that a sector was recovered.
- Persist recovered image data successfully before committing a map transition that claims that data exists. UI progress must reflect durable recovery state, not merely completed device I/O.
- Keep `recovered`, `deferred`, `unreadable`, and `uncovered` states distinct. Do not convert a timeout or first read failure directly into permanent unreadability unless the configured retry/finalization policy is exhausted.
- Recovery is coverage-first across the known LBA range. Repeated media read failures alone are not a reason to abort the entire job while later LBAs remain addressable. Fatal job failures are reserved for conditions that make safe continued coverage impossible, such as confirmed device loss, unrecoverable permission/access failure, unwritable destination/ENOSPC, persistence failure, invalid durable state, or explicit stop/termination semantics.
- A retry continues the same compatible image + `.drmap` pair, preserves sectors already recovered, and starts a new bounded retry cycle over unresolved work. Do not unnecessarily reread or overwrite already recovered sectors.
- Every read, retry, wait, queue, pass, split, probe, reopen, speed change, cancellation, pause, stop, and shutdown path has an explicit finite bound or deterministic termination condition.
- A timed-out or canceled raw-device request must not be abandoned as a leaked background read while a new request starts against the same source. Keep source-device I/O serialized and allow at most one active raw-device operation per recovery source unless an approved design explicitly proves otherwise.
- Pause/stop prevents new reads promptly, has a bounded escalation path for an in-flight request, and preserves a resumable image/map pair whenever one was validly established and safe persistence remains possible.
- Preserve native/platform failure context. Typed errors should retain the operation, device path, and underlying OS/device detail needed to distinguish no media, not ready, permission, busy, unavailable, media damage, timeout, and genuine device failure.
- Resume and automatic mutation of prior work require compatible durable state and sufficiently strong logical-content identity. Geometry, layout, or overlapping-sample conflicts block automatic mutation of an existing job.
- Logical-content, capture, device, user-label, and optional physical-copy identities remain separate. Drive identity and volume labels do not establish content identity.
- Recovery confidence is explicit. A successful read, repeated agreement, cross-capture agreement, trusted checksum, reconstruction, tentative data, and conflict are not interchangeable states.
- Catalog/history state is local and non-authoritative for recovery. Catalog failure or lock contention may warn, but must not invalidate an otherwise safe recoverable image/map pair.
- No unbounded goroutines, channels, read payloads, buffers, logs, journals, snapshots, retry loops, or retained device requests.
- Logs must not write to stdout while the TUI owns the terminal. Restore terminal state after normal exit, cancellation, signals, and handled fatal errors.

When safety and throughput conflict, preserve read-only source access, correct sector placement, durable resumability, bounded device ownership, and truthful status before optimizing speed.

## Architecture and ownership

Keep dependencies directed through project-owned boundaries. The current implementation uses shared recovery semantics with platform-specific optical adapters; do not reintroduce separate user-visible recovery semantics per operating system.

Use these package responsibilities as a working map and verify them against the current tree before editing:

- `cmd/discrescue`: executable entrypoint and internal modes used by the current runtime.
- `internal/app`: Bubble Tea model, messages, update/view logic, Bubbles component ownership, keys/help, and presentation-only projections. It must not perform blocking device I/O.
- `internal/coordinator`: authoritative job orchestration, transitions, cancellation/checkpoint coordination, and shutdown where the current path uses it.
- `internal/recovery`: platform-neutral recovery policies, scheduling, lifecycle rules, coverage/retry behavior, and deterministic planning.
- `internal/platform`: project-owned OS/runtime adapters, including filesystem/clock/process/terminal boundaries and the current platform-specific optical discovery, media inspection, capability, raw-device/recovery, and eject integration points.
- `internal/device`: project-owned device-domain types, bounded protocol/supervision pieces, media/capability modeling, and low-level device behavior used by current paths.
- `internal/device/linux`: Linux-specific block/`SG_IO`/MMC behavior where used.
- `internal/image`: positioned sector writes and image representation.
- `internal/mapfile`: `.drmap` header, checkpoint, journal, replay, extent state, and durable recovery-map rules.
- `internal/catalog`: logical-content identity lookup and bounded local history/catalog state.
- `internal/integrity` and `internal/merge`: verification, reconstruction/consensus boundaries, confidence, and provenance.
- `internal/logging`: bounded structured event records and redaction.
- `internal/testdevice`: deterministic simulator, fault scripts, command audits, and release-hardening scenarios.
- `internal/buildinfo`: embedded version/build metadata.

`docs/architecture/package-layout.md` is a useful repository map, but audit it against the current source tree when touching architecture because documentation can lag implementation. If it is independently stale and outside the issue scope, file/link a follow-up rather than silently broadening the change.

Keep these boundary rules:

- The TUI never talks directly to native device handles.
- Potentially blocking device/filesystem/process work does not run inside Bubble Tea `Update` or `View`.
- Platform-neutral recovery policy, durability ordering, lifecycle state, and TUI semantics stay separate from OS-specific device discovery, ioctls, raw paths, and eject mechanics.
- Native capability support is reported explicitly instead of assumed from platform name.
- Interfaces belong at real effect, ownership, or test seams; do not introduce abstraction layers without a concrete alternate implementation or boundary need.
- Keep packages internal before v1.0 unless a genuine public API requirement is approved.

## State and effect design

Apply The Elm Architecture to interactive and job-control flows:

- `Model` contains the state needed to choose the next transition and render the current view. Recompute cheap derived values instead of storing contradictory copies.
- Messages describe events or intent, such as `SaveRequested` or `ReadCompleted`; avoid field-setter messages such as `SetSavingTrue`.
- `Update(message, model)` remains deterministic at the application-state layer and returns the next state plus effect descriptions/commands. Do not perform raw device I/O, filesystem access, hashing, sleeping, subprocess waiting, clock reads, or blocking channel receives in Bubble Tea `Update`.
- `View(model)` is a pure projection. Event handlers emit messages and do not mutate shared state directly.
- Every asynchronous effect reports success or failure through a typed message. Use request/generation IDs when stale results could overwrite newer intent.
- Represent mutually exclusive workflow states with explicit enums/tagged structures instead of contradictory boolean clusters.
- Test transitions including invalid, duplicate, stale, cancellation, timeout, retry, and failure sequences.

Prefer explicit data for sector extents, requests, results, snapshots, protocol frames, journal records, catalog events, configuration, capabilities, and status. Separate mutation into **calculate -> validate -> commit -> publish effects**. Validate all untrusted boundaries before allocation or state transition.

## Code quality rules

- Optimize in this order: correctness, safety, clarity, testability, then measured performance.
- Use intention-revealing, unambiguous, searchable domain names and current repository terminology. Pick one term per concept and avoid cute names, unnecessary encodings, or misleading familiarity.
- Keep functions/types cohesive and at a consistent level of abstraction. Prefer small units when they improve comprehension, but do not chase arbitrary line-count limits.
- Prefer explicit control flow over clever generic machinery. Reduce deep nesting with guard clauses, decomposition, or clearer state modeling when doing so preserves semantics.
- Extract repeated business meaning, not coincidental syntax.
- Make side effects and mutation obvious. Avoid output parameters and caller-owned mutation when a returned value or owned copy is clearer.
- Preserve error context instead of logging and swallowing it. Include operation/device/LBA/count/retryability/native error/map sequence/request identity where applicable.
- Wrap OS, filesystem, terminal, process, and third-party APIs behind small project-owned adapters. Add focused learning/contract tests when an external API or OS semantic is unfamiliar or version-sensitive.
- Validate protocol and persisted lengths/ranges before allocating. Encapsulate boundary conditions and keep configurable policy at the appropriate high level.
- Comments explain intent, invariants, public contracts, public-spec references, unusual ordering, and safety constraints. Do not narrate obvious code, duplicate the implementation, keep obsolete comments, or retain commented-out implementations.
- Avoid mutable globals, hidden clocks/randomness, panic as routine control flow, and background goroutines without a clear owner.
- A goroutine must have an owner, context/cancellation path, bounded input, error path, and shutdown position. Keep synchronized/critical sections small and do not spread shared mutable state across goroutines.
- Use the standard library unless a dependency materially reduces risk or implements a required public API.
- Measure before adding healthy-media complexity. Never trade crash ordering or bounded device ownership for throughput.

## Human-centric code contracts and error design

Code is maintained by humans under limited attention and memory. Optimize the repository for the next reader and caller, not merely for the compiler or original author.

### Reduce cognitive load

- Minimize the amount of non-local context a contributor must remember to understand or change a behavior.
- Prefer familiar Go/domain concepts, descriptive searchable names, shallow control flow, cohesive functions, and clear ownership boundaries.
- Avoid mental mapping, deceptive names, unexplained abbreviations, magic values, and abstractions whose benefit is smaller than their learning cost.
- Keep code at one useful level of abstraction at a time. High-level orchestration should read as orchestration; low-level encoding/I/O details belong behind the owning boundary.
- Do not optimize for minimum line count. Slightly more explicit code is preferable when it substantially improves comprehension.
- Essential knowledge must live in code, tests, ADRs, or repository documentation - not only in chat, issue comments, memory, or oral explanation.

### Make contracts visible

Callers should be able to infer correct use from names, types, and nearby contract documentation. Do not rely on hidden small print.

- Use descriptive types instead of overly general `string`, integer, tuple-like, or map values when misuse would be easy or safety-relevant.
- Keep function inputs focused on what the function actually needs.
- Make side effects, ownership transfer, mutation, blocking behavior, cancellation, and durability consequences obvious at the call boundary.
- Prefer one source of truth for both data and business logic. Secondary summaries/caches must be derived or have an explicit synchronization contract.
- Use comments for intent, rationale, invariants, external-spec references, warnings, and non-obvious consequences; comments do not excuse unclear names or structure.

### Error contract and recoverability

Classify failures according to what the immediate caller can safely do next. Only the caller may know whether a condition is recoverable in its context, so lower layers must preserve enough information to make that decision.

- Fail near the violated invariant or invalid boundary rather than letting corrupt or ambiguous state propagate.
- Do not hide errors, silently substitute success, or encode failures as undocumented magic values.
- Recoverable conditions must be explicit enough for the caller to choose a retry, defer, refresh, re-open, ask-user, or abort action.
- Unrecoverable conditions must preserve context and terminate the affected operation deterministically.
- Translate errors at abstraction boundaries only when the higher-level error adds caller-relevant meaning; preserve the underlying cause for diagnostics.
- Compiler/vet/static-analysis warnings relevant to changed code are defects to understand, not noise to suppress without rationale.

### Root-cause defect fixes

For a bug, fix the observed failure and the enabling condition when practical. Do not stop an RCA at `user error`, `caller misuse`, `developer mistake`, or `the OS returned an error`. Ask why the design permitted the failure and whether the invariant can be encoded, validated, isolated, or tested.

Use a short Five-Whys-style analysis when the cause is not obvious:

1. What directly failed?
2. Why could that failure reach the user or durable state?
3. Which missing/weak boundary, invariant, ownership rule, or test allowed it?
4. Could the same class of failure occur elsewhere?
5. What smallest design/test change prevents recurrence without broadening scope?

Add regression coverage for the failure class, not only the exact observed input.

### Local improvement without scope creep

Leave code you already touch at least as understandable as you found it. Small local cleanup of misleading names, dead code, trivial duplication, or avoidable complexity is encouraged when it reduces risk and does not change unrelated behavior. Larger cleanup/refactoring belongs in its own issue.

## Charm v2 TUI contract

The TUI is one persistent Bubble Tea v2 application, not a chain of shell prompts.

- Use `charm.land/bubbletea/v2` as the application event loop and state/message/command runtime.
- Use `charm.land/bubbles/v2` for reusable interactive behavior where appropriate, including authoritative key bindings/help, lists, spinners, text input, viewport behavior, and progress components.
- Capture and propagate child component `Update` results and commands; do not call child updates and discard their commands.
- Use `charm.land/lipgloss/v2` for semantic styling, spacing, borders, wrapping, and terminal-cell-aware measurement/layout.
- Do not mix Charm v1 and v2 APIs.
- Gum is optional developer/demo workflow tooling only. It is never a runtime replacement for the persistent Bubble Tea application.
- Keep visible help generated from or synchronized with the same active bindings used by `Update`. Do not advertise disabled or unavailable actions.
- Exactly one interactive child component owns focus at a time. Focus/blur transitions must be deterministic and tested.
- Keep alternate-screen mode enabled for the entire interactive DiscRescue session, including discovery, drive selection, setup, recovery, details, history, and summary pages. Restore the original terminal on exit.
- All supported actions must remain usable by keyboard. Color may reinforce meaning but cannot be the only carrier of meaning.
- Preserve Unicode, combining-character, ANSI-styled-text, long-path, and terminal-cell alignment behavior.
- Preserve representative responsive behavior at `120x36`, `80x24`, `60x18`, and `40x12`. Smaller supported sizes must not silently fall back to an obsolete layout. Below the supported minimum, prefer a bounded resize-required view rather than corrupt layout.
- Resizing must not interrupt or restart a recovery job.
- Keep recovery status truthful: recovered, deferred, unreadable, remaining/uncovered, paused/stopped, and fatal states must not be visually conflated.
- Technical detail stays secondary to the next safe action. Do not add decorative telemetry that obscures recovery state.

## Persistence, formats, and compatibility

Every project-owned persisted format or IPC protocol must have a versioned specification under `docs/formats/`. Change the specification and implementation together.

A format/protocol specification must define, as applicable:

- magic/version and byte order;
- field sizes and canonical encodings;
- maximum record and payload lengths;
- integrity/CRC behavior;
- unknown record/version handling;
- compatibility and migration policy;
- commit, flush, compaction, and crash-recovery semantics;
- truncation, corruption, deletion, and partial-write behavior.

Parsers must reject overflow, impossible lengths, invalid ranges, overlapping extents, and incompatible identity before allocation or mutation. Replay may tolerate only the truncation/corruption cases explicitly permitted by the format specification. Preserve older readers or provide an explicit migration when changing a frozen format version.

When a persisted field, journal/checkpoint rule, protocol frame, or compatibility guarantee changes, update `docs/formats/`, affected tests, and an ADR when the decision changes a durable contract.

## Test-driven development and verification

Use the deterministic simulator as the default development boundary. Real optical hardware complements simulator coverage; it does not replace it.

For each behavior change:

1. Reproduce or express the issue as the smallest deterministic unit, transition, simulator, golden, integration, or fault-injection test.
2. Test observable behavior/invariants rather than mirroring the current private implementation. Do not make an internal detail public solely so a test can call it; test through the owning boundary or extract a genuinely meaningful unit.
3. Keep each test focused enough that a failure clearly identifies the broken behavior. Use table/parameterized tests when many inputs exercise the same contract.
4. Implement only enough behavior to satisfy the intended contract while preserving the invariants above.
5. Refactor under green tests; tests should remain understandable, deterministic, independent of order, and quick enough for the intended validation tier.
6. Run focused affected-package tests during iteration.
7. Run the repository-wide validation appropriate to the change risk before handoff.

The repository-owned Go devtool is the canonical shell-neutral validation interface on Linux, macOS, and Windows:

```text
go run ./tools/devtool format --check
go run ./tools/devtool test
go run ./tools/devtool check
go run ./tools/devtool release --race=auto
```

Use `go run ./tools/devtool format` when formatting changes are required. Use direct Go commands such as focused `go test`, `go test -race`, `go vet`, and `go build -trimpath ./cmd/discrescue` as targeted diagnostics or supplemental verification; they do not replace the canonical devtool workflow for repository-wide validation.

Apply risk-appropriate emphasis:

- recovery/persistence/protocol changes: corruption, truncation, replay, restart/resume, checkpoint ordering, ENOSPC, and failure injection;
- device/lifecycle changes: blocked/slow reads, timeout/cancel, serialized ownership, device disappearance, media replacement, and native error classification;
- TUI changes: transitions, stale async results, authoritative help, focus ownership, alternate-screen behavior, Unicode/monochrome layouts, and representative terminal sizes;
- concurrency/performance changes: race, leak, soak, throughput, and CPU checks;
- format/protocol changes: fixed vectors, bounds, compatibility, unknown versions/records, and crash points.
- external/OS/third-party boundary changes: focused learning/contract tests that pin the behavior DiscRescue relies on without leaking the dependency through the rest of the codebase.

### Evidence levels

Report validation evidence accurately:

- **Portable/simulator evidence** proves shared logic, TUI behavior, formats, command audits, and deterministic recovery scenarios that do not require a target device.
- **Native evidence** proves code selected and run on the target operating system/toolchain, including terminal behavior and race validation where supported.
- **Cross-compilation evidence** proves build selection/compile compatibility only. It supplements native testing; it does not replace it.
- **Hardware evidence** requires a real optical drive/media path on the target operating system. Do not claim discovery, raw-device access, eject, timeout/cancellation, or bridge behavior from simulator tests or cross-compilation alone.

If race validation is unavailable and the devtool reports `SKIPPED`, report it as skipped, not passed.

For documentation-only changes, minimum verification is `git diff --check` plus validation that every referenced repository path and command is current. Run broader Go checks if the documentation audit uncovers or changes executable behavior, generated configuration, or validation tooling.

## Workspace and artifact hygiene

Protect user work and recovery artifacts from unrelated changes.

Do not add, delete, rename, reset, clean, stash, overwrite, or commit unrelated local artifacts merely because they are present. In particular, leave these untouched unless the active issue explicitly requires them:

- built `discrescue` or `discrescue.exe` binaries;
- recovered `.iso`, `.bin`, `.cue`, or other image files;
- `.drmap` recovery maps and checkpoints;
- local screenshots, terminal captures, traces, and logs;
- downloaded media, test discs, personal volume labels, and other user-generated files;
- unrelated modified or untracked source files.

If an issue requires a tracked fixture or artifact, use synthetic, bounded, non-sensitive data. Never commit real recovered disc contents, secrets, personal labels, or sensitive logs.

Before committing, inspect the diff/status and stage only the intended issue-scoped files.

## Documentation rules

- Keep architecture contracts under `docs/architecture/`.
- Keep accepted decisions under `docs/adr/`.
- Keep owned persisted-format/protocol specifications under `docs/formats/`.
- Update documentation in the same change as an interface, invariant, command set, persisted field, state transition, default, platform capability, or user-visible workflow when that documentation would otherwise become inaccurate.
- ADRs should record context, alternatives, decision, consequences, migration/rollback considerations, and verification.
- Exported Go declarations receive concise contract documentation. Internal comments focus on non-obvious intent and safety constraints.
- User documentation must distinguish completed, verified, incomplete, deferred, unreadable, and resumable states accurately and must not overclaim physical identity or recovery confidence.
- Examples use synthetic paths, identifiers, sector data, and device metadata.

## Completion checklist

A change is complete only when all applicable items are true:

- The diff is scoped to the active issue and acceptance criteria.
- Non-trivial design choices identify the current requirement, relevant trade-offs/unknowns, and evidence; speculative flexibility/performance is not introduced without a requirement.
- SRP/DRY/KISS/YAGNI/SOLID and related design principles are followed, or an intentional exception is explained where the design is reviewed.
- Names/types/contracts minimize cognitive load and make unsafe/invalid states difficult to express where practical.
- Referenced architecture, ADR, format, and repository paths actually exist or are updated in the same change.
- Authoritative state has one clear owner; effects, device ownership, timeouts, retries, and cancellation boundaries are explicit.
- Source access remains read-only and all new loops, queues, payloads, waits, timeouts, retries, and goroutines are bounded.
- Coverage-first recovery, durable image-before-map ordering, resumability, identity compatibility, and catalog independence are preserved where applicable.
- Normal, failure, cancellation, timeout, stale-result, retry, and crash/replay paths are tested at the appropriate layer.
- TUI bindings/help/focus are consistent, alternate-screen ownership is preserved, and supported Unicode/monochrome/responsive layouts remain usable.
- Format/protocol and architecture documentation matches the implementation.
- Canonical devtool validation and any risk-specific checks have passed, or omitted/skipped checks are explicitly reported with the reason.
- Hardware claims are backed by real target-hardware evidence.
- The final diff contains no unrelated rewrites, generated debris, recovery images/maps, sensitive data, or accidental changes to user work.
