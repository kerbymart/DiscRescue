# DiscRescue Worker Protocol v1

## Status

Draft for implementation

## Scope

This document defines the version 1 parent-worker binary protocol used between the coordinator-side supervisor and the self-executed device worker. The protocol is bounded, length-prefixed, and CRC-protected.

## Transport

- Linux target transport: Unix domain socketpair
- Local development transport: implementation-defined adapter that preserves this frame contract
- At most one active device command may exist per worker at a time

The protocol does not authorize the worker to open output files, update the recovery map, or control UI state.

## Frame Layout

Each frame is:

| Field | Type | Notes |
| --- | --- | --- |
| magic | 4 bytes ASCII | `DSWP` |
| version | uint16 | `1` |
| type | uint16 | message type |
| request_id | uint64 | non-zero, monotonically increasing per parent process |
| length | uint32 | payload byte length |
| payload | bytes | message-specific payload |
| crc32c | uint32 | CRC32C over all bytes before this field |

All integer fields are little-endian.

Validation rules:

- Reject unknown `magic`.
- Reject unsupported `version`.
- Reject `request_id == 0`.
- Reject payload lengths above `1 MiB`.
- Reject CRC mismatches.
- Reject frames shorter than the fixed header plus CRC32C.

## Message Types

| Code | Name | Direction |
| ---: | --- | --- |
| 1 | `HELLO` | worker to parent |
| 2 | `HELLO_ACK` | parent to worker |
| 3 | `OPEN_DEVICE` | parent to worker |
| 4 | `PROBE_MEDIA` | parent to worker |
| 5 | `READ_BLOCKS` | parent to worker |
| 6 | `SET_SPEED` | parent to worker |
| 7 | `TEST_READY` | parent to worker |
| 8 | `EJECT` | parent to worker |
| 9 | `CLOSE_DEVICE` | parent to worker |
| 10 | `CANCEL_CURRENT` | parent to worker |
| 11 | `HEARTBEAT` | worker to parent |
| 12 | `RESULT` | worker to parent |
| 13 | `ERROR` | worker to parent |
| 14 | `SHUTDOWN` | either direction |

Unknown message types are fatal protocol errors in v1.

## Common Payload Rules

- Strings are UTF-8 with explicit byte length prefixes inside the payload schema that uses them.
- Paths are parent-owned inputs only and must never be sourced from worker suggestions.
- Sector data payloads must fit within the global `1 MiB` frame payload bound.
- The worker must return complete sense tuples when available.
- Partial transfers must include both requested and actual sector counts.

## Handshake

Startup sequence:

1. Worker sends `HELLO` with protocol version and worker instance ID.
2. Parent validates the version and replies with `HELLO_ACK`.
3. Parent may then send device commands.

If the version is unsupported, the parent must reject the worker and stop the session.

## Representative Payload Schemas

### `HELLO`

| Field | Type | Notes |
| --- | --- | --- |
| protocol_version | uint16 | must match frame version |
| worker_id_length | uint16 | bounded UTF-8 length |
| worker_id | bytes | implementation instance ID |

### `OPEN_DEVICE`

| Field | Type | Notes |
| --- | --- | --- |
| device_path_length | uint16 | bounded UTF-8 length |
| device_path | bytes | parent-supplied path |
| require_optical | uint8 | `1` in v1 |
| require_lock | uint8 | advisory lock requested |

### `READ_BLOCKS`

| Field | Type | Notes |
| --- | --- | --- |
| start_lba | uint64 | inclusive |
| sector_count | uint32 | bounded by read policy |
| logical_sector_size | uint32 | expected sector size |
| soft_deadline_ms | uint32 | parent-side reporting budget |
| hard_deadline_ms | uint32 | parent-side termination budget |
| retry_budget | uint16 | request retry budget |

### `RESULT`

| Field | Type | Notes |
| --- | --- | --- |
| result_type | uint16 | operation-specific result discriminator |
| status_code | uint16 | `0` for success in v1 |
| actual_sector_count | uint32 | sectors returned or affected |
| sense_key | uint8 | `0` when unavailable |
| asc | uint8 | `0` when unavailable |
| ascq | uint8 | `0` when unavailable |
| reserved | uint8 | zero in v1 |
| payload_length | uint32 | nested payload length |
| payload | bytes | operation-specific body |

### `ERROR`

| Field | Type | Notes |
| --- | --- | --- |
| error_class | uint16 | retryable or fatal class |
| os_error_length | uint16 | bounded UTF-8 length |
| os_error | bytes | human-readable OS error |
| sense_key | uint8 | `0` when unavailable |
| asc | uint8 | `0` when unavailable |
| ascq | uint8 | `0` when unavailable |

## Deadlines

Each request carries or maps to:

- a parent-side soft deadline
- a parent-side hard deadline
- a worker-side device timeout where supported
- a retry budget

Default policy examples from the TDD:

| Operation | Soft deadline | Hard deadline |
| --- | ---: | ---: |
| Inquiry or probe | 10 s | 30 s |
| Healthy cluster read | 15 s | 45 s |
| Single damaged sector read | 30 s | 120 s |
| Set speed | 5 s | 15 s |
| Eject | 10 s | 30 s |

Soft deadline expiry updates the UI and blocks new scheduling. Hard deadline expiry triggers cancellation and then worker termination.

## Worker Constraints

The worker may:

- open the source read-only
- probe media
- read sectors
- adjust speed according to bounded policy
- emit heartbeat frames between commands

The worker must not:

- open the output image
- update map or catalog state
- render UI
- mount or unmount media
- issue write, format, blank, close-track, or reserve-track commands
- reset a SCSI bus or controller by default

## Failure Handling

- A CRC mismatch is a fatal session error.
- An unsupported version is a fatal handshake error.
- A hard deadline breach results in `CANCEL_CURRENT`, followed by process termination if needed.
- The parent must ignore stale `RESULT` or `ERROR` frames whose `request_id` no longer matches the active request.

## Compatibility

- v1 readers and writers must reject unknown frame versions.
- v1 does not define extension frames or optional trailing fields.
- Future versions may add message types only with a version bump or a negotiated capability scheme.
