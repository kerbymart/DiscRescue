# macOS Development and Hardware Validation

DiscRescue supports macOS optical media through a narrow `diskutil` adapter in `internal/platform`. Discovery uses `diskutil list`; media inspection uses `diskutil info`; recovery opens the whole-disk raw node (`/dev/rdiskN`) with read-only source semantics. The recovery map, positioned image writes, bounded retry passes, pause, cancellation, and resume checks remain project-owned behavior.

The adapter does not invoke a shell, unmount a volume, eject media, change device state, or issue a write to the source device. Command execution has a finite timeout. A macOS permission or media-access failure is returned as an actionable inspection or source-open error.

## Manual USB optical-drive check

On a macOS machine with a USB CD/DVD drive:

1. Insert a synthetic or otherwise non-sensitive test disc and confirm `diskutil list` shows a whole optical disk such as `/dev/disk4`.
2. Build and run `go run ./cmd/discrescue` from a terminal with the permissions needed to read the raw device.
3. Confirm the drive appears in drive selection, media inspection reports a non-zero sector size and sector count, and the suggested output is an ISO path.
4. Start a recovery into a directory with sufficient free space. Confirm the source path is the matching `/dev/rdiskN`, the ISO and `.drmap` are created, and progress advances.
5. Pause and stop after a checkpoint, restart DiscRescue, select the same media and output pair, and confirm the recovery is offered as resumable.
6. Repeat with raw-device access denied and confirm the UI reports an actionable permission error without ejecting or unmounting the disc.

Record the macOS version, machine model, USB optical-drive model, media geometry, and whether raw-device permission was granted. Do not commit disc contents, labels, or diagnostic output containing personal data.
