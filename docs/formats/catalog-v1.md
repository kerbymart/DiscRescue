# DiscRescue Catalog Format v1

## Status

Draft for implementation

## Scope

This document defines the version 1 local processed-media catalog format, journal, snapshot, locking rules, compaction rules, and crash-recovery behavior.

## Location

Default location:

```text
$XDG_STATE_HOME/discrescue/catalog/
    catalog.snapshot
    catalog.journal
    lock
```

Fallback when `XDG_STATE_HOME` is unset:

```text
~/.local/state/discrescue/catalog/
```

## Storage Model

The catalog consists of:

- an append-only journal
- an atomically replaced compacted snapshot
- an advisory lock file for mutation

The catalog is local-only, bounded, crash-safe, and non-authoritative for recovery correctness.

## Journal Record Envelope

Each journal record is:

| Field | Type | Notes |
| --- | --- | --- |
| magic | 4 bytes ASCII | `DSCJ` |
| version | uint16 | `1` |
| record_type | uint16 | see event table |
| sequence | uint64 | strictly increasing |
| payload_length | uint32 | bounded payload size |
| payload | bytes | event-specific body |
| crc32c | uint32 | CRC32C over all bytes before this field |

All integer fields are little-endian.

Unknown versions or record types are incompatible for v1 readers.

## Required Event Types

| Code | Name |
| ---: | --- |
| 1 | `MEDIA_OBSERVED` |
| 2 | `CAPTURE_STARTED` |
| 3 | `JOB_LINKED` |
| 4 | `JOB_STATE_CHANGED` |
| 5 | `FULL_CONTENT_HASH_ADDED` |
| 6 | `PHYSICAL_LABEL_SET` |
| 7 | `PATH_RELOCATED` |
| 8 | `RECORD_HIDDEN` |
| 9 | `SNAPSHOT_COMMITTED` |

## Snapshot File

The snapshot file is a complete serialized catalog state with this envelope:

| Field | Type | Notes |
| --- | --- | --- |
| magic | 4 bytes ASCII | `DSCS` |
| version | uint16 | `1` |
| payload_length | uint32 | snapshot payload bytes |
| last_sequence | uint64 | newest journal sequence included |
| record_count | uint32 | number of top-level records |
| payload | bytes | full catalog payload |
| crc32c | uint32 | CRC32C over all bytes before this field |

Writers must write snapshots to a temporary file, sync them, and atomically rename into place.

## Logical Records

Each top-level catalog record contains:

| Field | Type | Notes |
| --- | --- | --- |
| record_id | 16 bytes | UUID bytes |
| identity | embedded | content identity record |
| state | uint16 | processing state |
| first_seen_unix_nano | int64 | UTC timestamp |
| last_seen_unix_nano | int64 | UTC timestamp |
| last_processed_present | uint8 | `0` or `1` |
| last_processed_unix_nano | int64 | zero when absent |
| capture_count | uint16 | bounded capture list |
| job_count | uint16 | bounded job reference list |
| preferred_job_id | 16 bytes | zero UUID when absent |
| hidden | uint8 | `0` visible, `1` hidden |

Job references are advisory path records, not proof that referenced files still exist.

Capture records are serialized inline and contain:

- `capture_id` as a length-prefixed UTF-8 string
- device identity strings for vendor, product, revision, serial, and transport
- `started_at_unix_nano` as `int64`
- `user_label` as a length-prefixed UTF-8 string
- `physical_copy_present` as `uint8`
- optional physical-copy strings for `asset_id` and `hub_code_note`

Job reference records are serialized inline and contain:

- `job_id` as 16 bytes
- `path` as a length-prefixed UTF-8 string
- `files_present` as `uint8`

## Processing State Encoding

| Code | Name |
| ---: | --- |
| 0 | `observed` |
| 1 | `in_progress` |
| 2 | `stopped_resumable` |
| 3 | `completed_verified` |
| 4 | `completed_with_gaps` |
| 5 | `failed` |
| 6 | `merged` |

## Write Order

Catalog write order is:

1. Append a length-delimited journal record with sequence and CRC32C.
2. Flush according to durability policy.
3. Apply the event to the in-memory index.
4. Periodically write a full snapshot to a temporary file.
5. Sync and atomically rename the snapshot.
6. Truncate or rotate only journal entries already covered by the durable snapshot.

## Locking

- Mutation requires an exclusive advisory lock on `lock`.
- A second DiscRescue process may open the catalog read-only.
- Read-only access must report that history updates are temporarily unavailable.

## Crash Recovery

Startup rules:

1. Attempt to read the snapshot.
2. If the snapshot is valid, use its `last_sequence` as the replay base.
3. Replay valid journal records with higher sequence numbers.
4. Ignore a truncated final journal record at EOF.
5. Stop replay on any earlier corruption and preserve prior durable history.

Catalog failure must warn but must not abort a safe recovery run.

## Compaction and Bounds

- Full sector hashes do not belong in the catalog except the optional full-content digest field.
- Event logs do not belong in the catalog.
- The journal must be compacted periodically to keep local storage bounded.
- Capture and job-reference lists may be bounded by policy, but older historical state must not be misrepresented as current availability.

## Privacy and Deletion Semantics

- The catalog is local-only by default.
- Paths may be updated by relocation events without deleting historical processing state.
- Hidden records remain structurally present until compaction policy removes them according to the defined retention policy.

## Compatibility

- v1 readers reject unknown versions.
- v1 writers must zero reserved fields if later added.
- Snapshot and journal versions must advance together if their shared logical model becomes incompatible.
