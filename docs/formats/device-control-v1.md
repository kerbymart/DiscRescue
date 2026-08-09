# Device control contract v1

Epic 5 device controls use project-owned typed requests. A read-speed request is
either `auto` (the default) or an explicit positive speed in kilobytes per
second. The worker payload is exactly eight bytes: a one-byte mode (`0` auto,
`1` explicit), three reserved zero bytes, and a little-endian uint32 speed.

Requests are bounded and validated before dispatch. An explicit request with a
zero speed is rejected. The device supervisor must apply an explicit request
before the first recovery read; an application error aborts startup rather than
silently continuing at an unknown speed. Options are scoped to the stable device
identity and current media token, so they are discarded after media replacement.

The enclosing worker frame remains version 1, length-prefixed, and CRC32C
protected as specified by `internal/device/protocol.go`.

Eject requests use a four-byte payload: mode (`0` normal, `1` force), explicit
confirmation (`0` or `1`), and two reserved zero bytes. Force eject is rejected
unless explicitly confirmed. Normal and force eject remain distinct operations;
the caller must release recovery ownership before normal eject and must verify
the resulting media state after either operation.
