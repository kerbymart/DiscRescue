# macOS eject probe

This standalone diagnostic intentionally does not import DiscRescue packages.
It distinguishes the raw `DKIOCEJECT` request from the verified macOS
raw-device `diskutil eject` operation.

```bash
go run ./tools/macos-eject-probe -device /dev/disk4
go run ./tools/macos-eject-probe -device /dev/disk4 -force
```

The second command uses the same `diskutil eject` request as the app after an
explicit force confirmation. It ejects the supplied removable disk; use only
a disposable test disc.
