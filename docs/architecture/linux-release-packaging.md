# Linux Release Packaging Notes

DiscRescue ships as a single `discrescue` executable. Linux release work must preserve read-only device access, terminal restoration, and the bounded worker/coordinator model defined by the TDD.

## Build Metadata

Release builds inject:

- version
- commit
- build date

The shell-neutral entrypoint for metadata-aware builds is:

- `go run ./tools/devtool build`

It sets linker variables for `discrescue/internal/buildinfo` and keeps `-trimpath` enabled.

## Static-Build Evaluation

A static build with `CGO_ENABLED=0` is preferred only if it does not weaken:

- Linux optical-device access;
- terminal restoration;
- subprocess and worker behavior.

Static linking is therefore an evaluation item, not an unconditional rule. A release handoff must state whether the tested Linux build used `CGO_ENABLED=0` or a dynamic build and why that choice preserved the production requirements.

## Linux Privilege Expectations

DiscRescue must not require broad root-only operation when narrower device permissions are sufficient.

Expected release guidance:

- open optical devices read-only;
- rely on device-node permissions or group membership where possible;
- do not require write access to the source device;
- do not use setuid packaging;
- do not auto-unmount media.

If a distribution package needs a udev rule, group membership, or other device-permission setup, document that as packaging guidance rather than baking it into the executable.

## Packaging Notes

Linux packaging should include:

- the `discrescue` executable;
- user-facing documentation or a pointer to it;
- any package notes needed for device-node permissions;
- release verification notes for native race coverage, simulator gates, and command-audit status.

The package must not add helpers that expand scope beyond the single executable model. Service units, shell wrappers, or desktop launchers are optional distribution extras and must not become required for normal recovery use.
