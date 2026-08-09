# DiscRescue Recovery Map Format v1

## Status

Draft for implementation

## Scope

This document defines the version 1 on-disk recovery map format used by DiscRescue `.drmap` files. The map is authoritative for recovery state. Image bytes alone never prove that a sector is recovered.

## Goals

- Preserve `image offset = LBA * logical sector size`
- Keep durable recovery state separate from image bytes
- Support bounded append, checkpoint, replay, truncation tolerance, and resumable recovery
- Encode confidence and extent state without depending on zero-filled image regions

## File Layout

The implemented v1 layout is:

```text
Header
One checkpoint
Append-only journal

The dual-checkpoint/footer layout remains a future design and is not emitted
or required by the current v1 implementation. Readers must not infer state
from image bytes.
```

All integer fields are little-endian.

## Header

Header fields in order:

| Field | Type | Notes |
| --- | --- | --- |
| magic | 4 bytes ASCII | `DSR1` |
| format_version | uint16 | `1` |
| header_length | uint16 | total header bytes including CRC32C |
| logical_sector_size | uint32 | bytes per logical sector |
| expected_sector_count | uint64 | expected capture capacity in sectors |
| output_format | uint16 | `1=iso`, `2=bincue` |
| identity_algorithm_version | uint16 | content identity algorithm version |
| layout_sha256 | 32 bytes | SHA-256 of media layout description |
| quick_content_id_present | uint8 | `0` or `1` |
| quick_content_id | 16 bytes | zeroed when absent |
| capture_id | 16 bytes | UUID bytes |
| catalog_record_id_present | uint8 | `0` or `1` |
| catalog_record_id | 16 bytes | UUID bytes, zeroed when absent |
| job_id | 16 bytes | UUID bytes |
| creation_time_unix_nano | int64 | UTC timestamp |
| clean_shutdown | uint8 | `0` or `1` |
| reserved | 7 bytes | must be zero in v1 |
| header_crc32c | uint32 | CRC32C over header bytes before this field |

Header validation rules:

- Reject unknown `magic`.
- Reject unsupported `format_version`.
- Reject `header_length` smaller than the fixed v1 header size.
- Reject `logical_sector_size == 0`.
- Reject `expected_sector_count == 0`.
- Reject `quick_content_id_present` and `catalog_record_id_present` values other than `0` or `1`.
- Reject non-zero reserved bytes.

## Checkpoint

The checkpoint is followed by the journal. It contains:

| Field | Type | Notes |
| --- | --- | --- |
| checkpoint_magic | 4 bytes ASCII | `DSCP` |
| checkpoint_version | uint16 | `1` |
| payload_length | uint32 | bytes after header and before CRC32C |
| last_sequence | uint64 | last fully applied journal sequence |
| extent_count | uint32 | number of extents in payload |
| extents | repeated | sorted non-overlapping extents |
| checkpoint_crc32c | uint32 | CRC32C over checkpoint bytes before this field |

Extent payload entry:

| Field | Type | Notes |
| --- | --- | --- |
| start_lba | uint64 | inclusive |
| sector_count | uint32 | must be greater than zero |
| state | uint8 | see sector-state table |
| confidence | uint8 | see confidence table |
| attempts | uint16 | bounded retry-attempt count |
| capture_id | uint32 | extent capture identifier |
| sense_key | uint8 | last device sense key |
| asc | uint8 | last device ASC |
| ascq | uint8 | last device ASCQ |
| data_hash | 16 bytes | optional sector-data hash |

The parser validates payload length, extent-count arithmetic, exact buffer
length, CRC32C, extent ordering, and finite decode limits before allocating the
extent slice. The default limits are 64 MiB per checkpoint payload and
1,048,576 extents.

## Journal

Every journal record is:

| Field | Type | Notes |
| --- | --- | --- |
| record_type | uint16 | see record type table |
| sequence | uint64 | strictly increasing by 1 |
| payload_length | uint32 | bounded by `1 MiB` in v1 |
| payload | bytes | record-specific payload |
| crc32c | uint32 | CRC32C over bytes before this field |

Record payloads must be self-contained. A reader must reject a record whose payload length exceeds the remaining file bytes except for one special case: a truncated final record at EOF may be ignored.

## Record Types

| Code | Name |
| ---: | --- |
| 1 | `JOB_CREATED` |
| 2 | `CAPTURE_OPENED` |
| 3 | `PASS_STARTED` |
| 4 | `DATA_WRITTEN` |
| 5 | `EXTENT_STATE_CHANGED` |
| 6 | `ERROR_RECORDED` |
| 7 | `MEDIA_REIDENTIFIED` |
| 8 | `CHECKPOINT_COMMITTED` |
| 9 | `PASS_FINISHED` |
| 10 | `JOB_STOPPED` |
| 11 | `JOB_COMPLETED` |

Unknown record types make the file incompatible with a v1 reader.

## Sector State Encoding

| Code | Name | Meaning |
| ---: | --- | --- |
| 0 | `unknown` | not yet classified |
| 1 | `queued` | transient scheduling state |
| 2 | `recovered` | durable data exists in the image |
| 3 | `missing` | retry work confirmed the sector is still unreadable |
| 9 | `skipped` | deferred for a later bounded retry pass |

On replay, any persisted `queued` extent is downgraded to `unknown`.
Later extent-state records replace any overlapping earlier range so deferred sectors can be narrowed during retry passes without rewriting healthy neighbors.

## Confidence Encoding

| Code | Name | Meaning |
| ---: | --- | --- |
| 1 | `transport` | data was read successfully |
| 2 | `verified` | data was confirmed by stronger verification |
| 3 | `conflict` | conflicting evidence exists |

Additional confidence states require a new format version or a compatible extension policy.

## Commit Ordering

For recovered data, the required order is:

1. Validate worker result bounds.
2. Write the full sector data to the image.
3. Confirm image write success.
4. Sync the image.
5. Append the extent transition to the journal.
6. Sync the journal immediately or within the bounded 8 MiB/256-record batch.
7. Only then expose the progress change to the UI as durable state.

For failed data:

1. Append the failed-attempt record.
2. Append the resulting extent-state transition.
3. Commit the journal.
4. During a fast pass, prefer `skipped` for a deferred retry range instead of immediately promoting the whole failed block to `missing`.
5. Queue a later pass only if policy allows.

## Replay and Crash Recovery

Startup rules:

1. Validate the header and checkpoint.
2. Replay only journal records with valid CRC32C and strictly increasing sequence numbers.
3. Ignore an incomplete final record at EOF, including a torn final header.
4. Stop replay on any other corruption and preserve earlier durable history.
5. Validate every extent's checked end against expected media capacity.
6. Convert any replayed `queued` state back to `unknown`.
7. Verify image length against the highest recovered extent.
8. Require media re-identification before resume.

## Compatibility

- A v1 reader must reject newer `format_version` values it does not understand.
- A v1 writer must rewrite all reserved bytes as zero.
- Backward-compatible additions may only use footer data or explicitly reserved bits that v1 readers already ignore safely.

## Size and Bounds

- Maximum journal payload length: `1 MiB`
- Maximum checkpoint payload: `64 MiB`
- Maximum extent count in one checkpoint: `1,048,576`
- Maximum pending journal batch: `8 MiB` or `256` records
- Journal growth is currently bounded by the recovery job's finite work and
  remains subject to future checkpoint/compaction work.

## Failure Semantics

- The map remains authoritative even if the image contains placeholder bytes.
- Deletion or corruption of the image does not authorize the map to claim recovered data.
- Deletion or corruption of the map prevents trusted resume until recovery logic revalidates state.
