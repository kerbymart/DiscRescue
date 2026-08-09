# Native platform parity

The project-owned `OpticalCapabilityProvider` reports operation-level support
without exposing native handles to the TUI or recovery packages.

| Platform | Discovery/media | Read-only recovery | Eject |
| --- | --- | --- | --- |
| Linux | `/dev/sr*` discovery and device access | Build-tagged raw optical adapter using the shared bounded recovery engine | Optical-drive ioctl |
| macOS | Pure-Go Darwin disk ioctl discovery | `/dev/rdiskN` read-only adapter | `DKIOCEJECT` ioctl |
| Windows | Win32 optical discovery and raw geometry | read-only volume adapter | storage eject device-control request |

Unsupported operations are reported at capability level. Native errors remain
typed and include the operation and device path. Shared recovery policy,
durable map ordering, lifecycle transitions, and UI state remain platform
neutral.

Hardware validation is still required on representative optical drives and
USB/SATA bridges on each target operating system; cross-platform compilation
does not substitute for that hardware evidence.
