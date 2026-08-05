# DiscRescue Event Log Format v1

## Status

Draft for implementation

## Scope

This document defines the version 1 JSON Lines event log used for bounded machine-readable recovery diagnostics.

## Storage Rules

- The event log is JSON Lines.
- The active TUI must never write logs to stdout.
- Each line is one UTF-8 JSON object.
- Newline characters inside string values must be escaped by the JSON encoder.

## Required Top-Level Fields

| Field | Type | Notes |
| --- | --- | --- |
| time | string | RFC 3339 or RFC 3339 with fractional seconds |
| level | string | `debug`, `info`, `warn`, or `error` |
| component | string | project-owned component name |
| event | string | event name |

Additional fields depend on the event type.

## Representative Record

```json
{"time":"2026-08-06T00:00:00.000+08:00","level":"warn","component":"device-worker","event":"read_failed","lba":183920,"count":16,"sense_key":3,"asc":17,"ascq":0,"attempt":2,"pass":"fast"}
```

## Required Event Coverage

The log must be able to represent:

- device attempts and failures
- sense data tuples
- selected policies and pass changes
- worker lifecycle events
- durable state transitions
- merge provenance decisions
- catalog warnings that do not stop recovery

## Redaction

- Sensitive filesystem paths may be redacted in exported diagnostic bundles.
- Redaction must preserve enough structure to show which field was removed.
- Raw secrets, personal labels, and unrelated user data must not be emitted.

## Validation Rules

- Invalid JSON lines are corrupt records.
- Unknown top-level fields are allowed.
- Missing required top-level fields make the record invalid.
- Numeric fields such as `lba`, `count`, `attempt`, `sense_key`, `asc`, and `ascq` must reject negative values.

## Rotation and Bounds

- The active in-memory event buffer must remain bounded.
- On-disk growth must be managed by file rotation or explicit archival policy before release.
- Rotation must not interfere with the current recovery run or terminal restoration.

## Compatibility

- v1 relies on field-name compatibility rather than a separate binary version tag.
- Required field meanings are stable once released.
- Future additions may add optional fields, but must not silently redefine existing ones.
