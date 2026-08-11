# ADR 0009: macOS confirmed force eject

## Status

Accepted

## Context

The original normal macOS eject used the public `DKIOCEJECT` ioctl. It cannot
eject a mounted optical volume, returning `EBUSY` before it can issue the eject
request. The force-eject UI was therefore misleading: it retried the same
ioctl with no different native behavior.

Hardware validation with an external USB optical drive established that
`diskutil eject /dev/rdiskN` ejects a mounted disc. `diskutil eject force
<disk>` is not valid syntax on the tested macOS release; an added
`unmountDisk force` operation is neither necessary nor part of the verified
working path.

## Decision

Use fixed `/usr/sbin/diskutil eject /dev/rdiskN` for normal macOS eject against
the normalized raw whole-device node under a ten-second deadline. Force eject
uses the same mechanism only after explicit confirmation, because macOS does
not expose a distinct public force-eject operation.

## Consequences

- Normal eject works for a mounted optical volume. Force confirmation remains
  available for the cross-platform UI contract, but it has no stronger macOS
  mechanism and is never a recovery side effect.
- This is the sole production host-command exception in the device path. The
  source-command audit permits it only in the Darwin eject adapter.
- A missing, timed-out, or failed command remains a typed eject error; the UI
  never claims ejection succeeded without an accepted operation and refresh.
- A future verified no-CGO Disk Arbitration binding may replace this exception.
