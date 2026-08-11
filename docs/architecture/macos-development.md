# macOS Development and Hardware Validation

DiscRescue supports macOS optical media through the pure-Go Darwin adapter in `internal/platform`. Automatic discovery probes whole `/dev/diskN` nodes but only publishes nodes with positive readable-media evidence; it never presents a permission-denied or otherwise unverified storage node as optical. `DISKRESCUE_DARWIN_OPTICAL_DEVICES` is an explicit optical-device override for vendor nodes and deterministic tests, so its configured paths may remain visible with actionable access errors. Eject uses the fixed, bounded `/usr/sbin/diskutil eject` request against the normalized raw device (`/dev/rdiskN`) so macOS can coordinate mounted optical volumes. Recovery opens that raw node with read-only source semantics. The recovery map, positioned image writes, bounded retry passes, pause, cancellation, and resume checks remain project-owned behavior.

The eject path is the sole production host-command exception: it invokes the absolute `/usr/sbin/diskutil` path with a normalized device argument and a finite timeout. Recovery, discovery, and media inspection do not invoke `diskutil`, `drutil`, or another shell command. The adapter uses no cgo or Apple SDK headers, so Darwin binaries can be cross-compiled from any Go-supported host. A macOS permission or media-access failure is returned as an actionable inspection or source-open error.

## Manual USB optical-drive check

On a macOS machine with a USB CD/DVD drive:

1. Insert a synthetic or otherwise non-sensitive test disc and confirm the DiscRescue drive chooser shows the Darwin-ioctl-discovered whole optical disk.
2. Build and run `go run ./cmd/discrescue` from a terminal with the permissions needed to read the raw device.
3. Confirm the drive appears in drive selection, media inspection reports a non-zero sector size and sector count, and the suggested output is an ISO path.
4. Start a recovery into a directory with sufficient free space. Confirm the source path is the matching `/dev/rdiskN`, the ISO and `.drmap` are created, and progress advances.
5. Pause and stop after a checkpoint, restart DiscRescue, select the same media and output pair, and confirm the recovery is offered as resumable.
6. Repeat with raw-device access denied and confirm the UI reports an actionable permission error without ejecting or unmounting the disc.

Record the macOS version, machine model, USB optical-drive model, media geometry, and whether raw-device permission was granted. Do not commit disc contents, labels, or diagnostic output containing personal data.
