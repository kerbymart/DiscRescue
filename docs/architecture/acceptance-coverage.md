# Acceptance Coverage Audit

Date of audit: 2026-08-05

This audit checks the current repository state against TDD Sections 22, 23, and 27. Status values are:

- Covered: current repository evidence directly supports the requirement.
- Partial: current evidence covers part of the requirement, but not the full stated scope.
- Gap: the repository does not yet contain enough current evidence to claim coverage.

## Section 22 Testing Strategy

| TDD area | Status | Current evidence | Notes |
| --- | --- | --- | --- |
| 22.1 Unit tests | Covered | `internal/mapfile/*_test.go`, `internal/catalog/*_test.go`, `internal/recovery/*_test.go`, `internal/integrity/*_test.go`, `internal/merge/*_test.go`, `internal/device/*_test.go` | Current repository state contains unit coverage for map replay, identity comparison, catalog behavior, retry budgets, confidence handling, merge, and verification boundaries. |
| 22.2 Device simulation | Partial | `internal/testdevice/simulator_test.go`, `internal/testdevice/audit_test.go`, `internal/testdevice/release_gate_test.go` | Scenario validation, read-only command audit, retry budgets, soak, and leak checks are present. Current repository evidence does not yet prove the full Linux CI statement or every end-to-end recovery algorithm running through simulator-backed integration in CI. |
| 22.3 Fault injection | Partial | `internal/image/enospc_test.go`, `internal/mapfile/replay_test.go`, `internal/coordinator/transition_table_test.go`, `internal/coordinator/shutdown_test.go` | Current tests cover ENOSPC, truncated replay, worker crash/hung-worker responsiveness, and terminal restoration ordering. Dedicated kill-point coverage for checkpoint rotation, TUI quit interruption, and every listed termination point is not yet fully evidenced. |
| 22.4 TUI tests | Partial | `internal/app/view_test.go`, `internal/app/update_test.go` | Current evidence covers the named screens, supported layout tiers, below-minimum resize, monochrome-safe rendering, wrapped paths, safe destructive defaults, footer keys, and no dense telemetry table. Explicit golden-view artifacts, invalid Unicode/device-string panic coverage, and some history-browser states are not yet directly evidenced. |
| 22.5 Linux integration tests | Gap | No current Linux hardware test artifacts in repository | This remains a later Linux-device milestone item. |
| 22.6 Soak tests | Partial | `internal/testdevice/release_gate_test.go`, `go run ./tools/devtool release` | Current evidence covers repeated simulator validation and a goroutine-leak guard. The TDD's 24-hour runs, thousands of worker restarts, and terminal attach/detach coverage are not yet present. |

## Section 23 Acceptance Criteria

### Functional

| Requirement | Status | Current evidence | Notes |
| --- | --- | --- | --- |
| Healthy DVD produces a byte-identical ISO | Partial | `internal/image/writer_test.go`, `internal/image/commit_order_test.go` | Current repository proves positioned ISO writes and sector alignment, but not a full end-to-end healthy DVD acceptance fixture. |
| Healthy data CD produces a byte-identical ISO | Partial | `internal/image/writer_test.go` | Same current scope as DVD ISO coverage. |
| Healthy audio CD produces a layout-correct BIN/CUE capture | Gap | `internal/image/bincue.go` exists, but no current end-to-end audio acceptance test | This remains a later raw/mixed-mode milestone item. |
| Failed cluster does not permanently hide readable neighboring sectors | Covered | `internal/recovery/fast_pass_test.go`, recovery scheduler tests | Current recovery tests cover failed-sector scheduling and neighboring-sector handling. |
| Interrupted job resumes without rereading verified sectors | Partial | `internal/coordinator/coordinator_test.go`, `internal/testdevice/audit_test.go` | Current evidence covers pause/resume transitions and audit boundaries, but not a full persisted resumable end-to-end acceptance scenario. |
| Wrong disc is rejected before output mutation | Partial | `internal/recovery/pass0_test.go`, `internal/catalog/lookup_test.go` | Current identity and bounded lookup tests cover mismatch detection, but not a full output-mutation acceptance flow. |
| Complementary captures can be merged conservatively | Covered | `internal/merge/merge_test.go` | Current merge planner covers identity blocking, overlap rejection, conservative selection, missing sectors, and explicit conflicts. |
| Every missing sector is represented in the map | Covered | `internal/merge/merge_test.go`, `internal/mapfile/replay_test.go` | Current merge and map tests preserve explicit missing/conflict states. |
| Output remains correctly sized and sector-aligned | Covered | `internal/image/writer_test.go`, `internal/image/commit_order_test.go` | Current repository verifies LBA-to-offset and positioned-write behavior. |
| Re-inserting strongly matching contents shows prior completed or resumable job before a new job starts | Covered | `internal/catalog/prior_processing_test.go`, `internal/app/view_test.go`, `internal/app/update_test.go` | Current catalog and TUI tests cover strong prior matches and safe default flows. |
| Same logical contents through a different drive produce the same quick content identity | Covered | `internal/catalog/fingerprint_test.go`, `internal/recovery/pass0_test.go` | Current identity tests exclude drive identity from content matching. |
| Two separate copies with identical logical contents share a content record but retain separate captures and physical labels | Partial | `internal/catalog/provenance_test.go`, `internal/catalog/persistence_test.go` | Current identity/capture separation is covered. A dedicated end-to-end two-copy catalog acceptance scenario is not yet explicit. |
| Matching volume label without matching layout/samples is not a content match | Covered | `internal/catalog/fingerprint_test.go`, `internal/catalog/lookup_test.go` | Current identity comparison is layout/sample based. |
| Damaged disc with insufficient readable samples produces indeterminate instead of false match | Covered | `internal/catalog/lookup_test.go`, `internal/recovery/pass0_test.go`, `internal/catalog/prior_processing_test.go` | Current lookup and TUI copy cover indeterminate results. |
| Stale archive path is shown as missing without deleting historical record | Partial | `internal/catalog/prior_processing_test.go`, `internal/catalog/store.go` | Current prior-processing copy covers unavailable paths, but not a full history-page acceptance flow. |
| Processed-media history page can locate records by label, hint, filename, date, and asset ID | Gap | No current history-browser search implementation evidence | This remains unfinished. |

### Reliability

| Requirement | Status | Current evidence | Notes |
| --- | --- | --- | --- |
| No recovery operation loops forever in simulator tests | Partial | `internal/testdevice/release_gate_test.go`, bounded scheduler/coordinator tests | Current bounded logic and soak validation help, but there is no full recovery-operation simulator loop acceptance harness yet. |
| Bubble Tea remains responsive during a hung simulated worker | Covered | `internal/app/update_test.go`, `internal/coordinator/transition_table_test.go`, `internal/testdevice/simulator_test.go` | Current repository covers hung-worker handling and TUI responsiveness behavior. |
| Parent memory remains within configured bound | Gap | No current memory-bound assertion in repository | This remains unproven. |
| Ctrl+C produces a valid checkpoint | Partial | `internal/app/update.go`, `internal/coordinator/coordinator_test.go`, `internal/coordinator/shutdown_test.go` | Current transition logic and shutdown ordering exist, but there is no direct acceptance test for the full Ctrl+C checkpoint path. |
| Forced worker crash does not corrupt the map | Partial | `internal/coordinator/transition_table_test.go`, `internal/mapfile/replay_test.go` | Current worker-crash and replay tests cover parts of the requirement, but not a direct persisted map-corruption acceptance test. |
| ENOSPC stops scheduling and preserves resumable state | Covered | `internal/image/enospc_test.go` | Current ENOSPC evaluation tests directly cover resumable-state preservation. |
| Terminal state is restored after handled fatal errors | Covered | `internal/coordinator/shutdown_test.go`, `internal/app/view_test.go` | Current shutdown ordering and TUI alt-screen behavior provide direct evidence. |
| No source-media write command appears in command-audit tests | Covered | `internal/testdevice/audit_test.go`, `go run ./tools/devtool release` | Current command-audit checks are explicit and part of the release gate. |
| Catalog corruption or lock contention does not stop a valid recovery | Partial | `internal/catalog/journal.go`, `internal/catalog/snapshot.go`, `internal/catalog/store.go`, current catalog tests | Current repository covers journal/snapshot validation and read-only open behavior, but not a full acceptance flow tied to an active recovery job. |
| Catalog replay never associates conflicting sample hashes as matching content | Covered | `internal/catalog/lookup_test.go`, `internal/catalog/fingerprint_test.go` | Conflict classification is directly covered. |

### Performance

| Requirement | Status | Current evidence | Notes |
| --- | --- | --- | --- |
| Healthy sequential imaging reaches the stated throughput target | Gap | `go run ./tools/devtool release`, `internal/merge/merge_bench_test.go`, `internal/integrity/verification_bench_test.go` | Benchmark commands now exist, but the TDD does not yet have an enforced repository threshold or healthy-imaging benchmark fixture. |
| UI rendering uses less than 5% CPU on a typical idle recovery screen | Gap | No current CPU-budget assertion in repository | Current release gate runs CPU-related benchmark commands only for merge and verification. |
| Progress messages are limited to the configured rate | Gap | No current progress-rate assertion in repository | This remains unproven. |
| Map commits are extent-based rather than per-sector during healthy reads | Covered | `internal/image/commit_order_test.go`, `internal/mapfile/extents_test.go` | Current repository uses extent-oriented state and positioned writes. |

## Section 27 Requirements Coverage Matrix

| Capability | Status | Current evidence | Notes |
| --- | --- | --- | --- |
| Continue after unreadable sectors | Covered | recovery scheduler tests, `internal/image/enospc_test.go` | Current scheduler and resumability tests support this capability. |
| Preserve exact image offsets | Covered | `internal/image/writer_test.go` | Direct byte-offset coverage exists. |
| Clustered reads with single-sector fallback | Partial | recovery scheduler tests | Current scheduler logic exists, but the exact injected cluster-failure acceptance trace is not isolated as a named acceptance artifact. |
| Skip difficult regions and revisit later | Covered | recovery scheduler tests and coordinator transitions | Current evidence supports deferred retry behavior. |
| Bounded retries and deadlines | Covered | `internal/testdevice/audit_test.go`, recovery tests, coordinator tests | Retry-budget evidence is explicit. |
| Durable recovery map | Covered | `internal/mapfile/replay_test.go`, `internal/integrity/verification_test.go` | Replay and verification coverage is direct. |
| Crash-safe data/map ordering | Covered | `internal/image/commit_order_test.go`, `internal/image/enospc_test.go` | Current durable ordering logic is tested. |
| Resume only the correct logical contents | Covered | `internal/catalog/lookup_test.go`, `internal/recovery/pass0_test.go`, `internal/catalog/prior_processing_test.go` | Identity mismatch and indeterminate handling are covered. |
| Recognize prior processing | Covered | `internal/catalog/prior_processing_test.go`, `internal/app/view_test.go` | Current evidence covers catalog lookup and TUI projection. |
| Keep drive identity out of content identity | Covered | `internal/catalog/provenance_test.go`, `internal/recovery/pass0_test.go` | Cross-drive separation is directly tested. |
| Separate identical physical copies as captures | Partial | `internal/catalog/provenance_test.go` | The identity boundary is covered; the full acceptance workflow remains partial. |
| Handle partial damaged-disc fingerprints | Covered | `internal/catalog/lookup_test.go`, `internal/recovery/pass0_test.go` | Current evidence is direct. |
| Persist local history safely | Covered | catalog journal/snapshot/persistence tests | Current repository has direct persistence coverage. |
| Resume without rereading good sectors | Partial | command-audit and coordinator tests | Explicit end-to-end persisted acceptance evidence is still partial. |
| Adaptive interval splitting | Covered | recovery scheduler tests | Current evidence is direct. |
| Reverse and scraping passes | Partial | scheduler vocabulary and retry tests | The current repository has bounded scrape modeling, but not a dedicated reverse-pass acceptance artifact. |

## Accepted Gaps

The remaining accepted gaps are recorded in `docs/adr/0002-acceptance-gap-audit-2026-08-05.md`.
