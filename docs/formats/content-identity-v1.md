# DiscRescue Content Identity Format v1

## Status

Draft for implementation

## Scope

This document defines the version 1 logical-content identity format and comparison rules used for catalog lookup, resume safety, and merge safety.

## Design Rules

- Content identity is stable across drives and captures.
- Device identity and capture identity are provenance only.
- User-supplied physical labels are separate from logical-content matching.
- Unavailable samples remain explicitly unavailable and are never represented as zero-filled hashes.

## Logical Model

The v1 content identity record contains:

| Field | Type | Notes |
| --- | --- | --- |
| version | uint16 | identity algorithm version |
| profile | uint16 | active media profile |
| logical_block_size | uint32 | bytes per logical sector |
| sector_count | uint64 | reported sector count |
| sessions | uint16 | session count |
| track_count | uint16 | number of track records |
| tracks | repeated | canonical ordered track entries |
| layout_sha256 | 32 bytes | canonical geometry and layout hash |
| volume_hint_count | uint16 | number of volume hints |
| volume_hints | repeated | bounded canonical string records |
| sample_count | uint16 | number of sample records |
| samples | repeated | bounded ordered sample records |
| quick_content_id_present | uint8 | `0` or `1` |
| quick_content_id | 32 bytes | zeroed when absent |
| full_content_sha256_present | uint8 | `0` or `1` |
| full_content_sha256 | 32 bytes | zeroed when absent |

All integer fields are little-endian.

## Canonical Inputs

The canonical identity encoding includes:

- active media profile
- logical block size
- reported sector count
- session count
- track start, end, mode, control flags, and lead-out positions
- readable volume identifiers and filesystem descriptors when available
- deterministic sector sample slots and hashes
- explicit availability state for every sample slot

## Track Entry

Each track entry contains:

| Field | Type | Notes |
| --- | --- | --- |
| track_number | uint16 | logical track number |
| start_lba | int64 | inclusive |
| end_lba | int64 | inclusive |
| mode | uint16 | canonical track mode |
| control_flags | uint16 | canonical control flags |
| lead_out_lba | int64 | track or session lead-out |

Track entries are ordered and must not overlap incompatibly.

## Volume Hint Entry

Each volume hint contains:

| Field | Type | Notes |
| --- | --- | --- |
| hint_type | uint16 | canonical hint category |
| length | uint16 | UTF-8 byte length |
| value | bytes | canonicalized UTF-8 |

Volume hints are optional and advisory. Unreadable hints are omitted rather than synthesized.

## Sample Entry

Each sample record contains:

| Field | Type | Notes |
| --- | --- | --- |
| slot | uint16 | deterministic slot number |
| lba | int64 | sample location |
| available | uint8 | `0` or `1` |
| sha256 | 32 bytes | zeroed when unavailable |
| error_class | uint16 | `0` when available |

If `available == 0`, the `sha256` field must be all zero bytes and the sample remains explicitly unavailable.

## Recommended Sample Slots

For data media:

- first readable data sector
- ISO9660 or UDF descriptor region when present
- deterministic positions near 12.5%, 25%, 50%, 75%, and 87.5%
- final readable data sector
- one extra deterministic slot derived from the canonical layout hash

For audio or mixed-mode CD:

- canonical TOC and lead-out layout
- beginning and end samples from each track where practical
- deterministic samples across the program area
- main-channel data only unless a later identity version expands scope

## Stable Identifiers

`layout_sha256` is always computed from canonical geometry and layout fields.

`quick_content_id` is present only when every mandatory sample slot for the identity version is readable:

```text
SHA-256(
    identity-version ||
    canonical-layout ||
    sorted mandatory sample records
)
```

`full_content_sha256` is optional and applies only to complete, gap-free captures:

- ISO or IMG output hashes all logical sectors in LBA order.
- BIN or CUE output hashes the canonical track layout followed by each captured main-channel sector in disc order.

## Comparison Results

The comparison result domain is:

| Code | Name | Meaning |
| ---: | --- | --- |
| 0 | `none` | no compatible candidate exists |
| 1 | `probable` | one to three independent samples match with no conflict |
| 2 | `strong` | quick ID matches, or layout plus at least four independent samples match |
| 3 | `indeterminate` | too few overlapping readable samples exist |
| 4 | `conflict` | geometry, layout, or overlapping readable sample differs |

Normative rules:

- `conflict` wins over every weaker result.
- Automatic resume and automatic merge require `strong`.
- `probable` requires explicit user confirmation.
- `indeterminate` must not auto-associate a disc with prior work.

## Capture and Device Provenance

The following are not part of logical-content scoring:

- device vendor, product, revision, serial, or transport
- capture ID
- user label
- physical asset ID
- hub or matrix code notes

These may be stored as provenance alongside the content identity.

## Validation Rules

- Reject unsupported identity versions.
- Reject zero logical block size or zero sector count.
- Reject duplicate sample slots.
- Reject impossible track ranges.
- Reject `quick_content_id_present` or `full_content_sha256_present` values other than `0` or `1`.
- Reject unknown mandatory fields missing for the declared version.

## Compatibility

- Newer identity versions must preserve the ability to compare or migrate older catalog records.
- Older readers must reject unsupported versions rather than reinterpret them.
- Threshold changes for sample counts are versioned with the identity algorithm.
