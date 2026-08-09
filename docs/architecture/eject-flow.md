# Optical eject flow

The drive chooser exposes normal eject (`e`) and force eject (`f`). Normal
eject is sent directly to the platform adapter. If it fails, the application
offers a separate force-eject confirmation page; force eject cannot be started
without explicit confirmation and is never used as recovery cancellation.

Platform adapters implement `platform.OpticalEjector` behind OS build tags:

- Linux uses the optical-drive eject ioctl after unlocking the door.
- macOS uses bounded `diskutil eject` invocation.
- Windows uses the storage eject device-control request.

Native success is reported as accepted-unverified unless the adapter can prove
media removal. Every accepted request triggers a fresh discovery pass, which
reconciles the selected drive and invalidates media-dependent state when the
media or drive is gone. Unsupported adapters return a typed unsupported error;
they are not presented as successful ejects.
