# DiscRescue Validation Workflow

The repository uses one canonical validation command on macOS, Linux, and
Windows:

```console
go run ./tools/devtool release --race=auto
```

GitHub Actions runs that command in a native three-OS matrix for pull requests
and pushes to `master`.

## Evidence matrix

| Verification area | macOS | Linux | Windows | Hardware required |
| --- | ---: | ---: | ---: | ---: |
| Format, vet, and package tests | Yes | Yes | Yes | No |
| Simulator recovery scenarios | Yes | Yes | Yes | No |
| Command audit | Yes | Yes | Yes | No |
| Soak and goroutine-leak checks | Yes | Yes | Yes | No |
| Merge/integrity benchmark smoke tests | Yes | Yes | Yes | No |
| Native adapter compilation | Yes | Yes | Yes | No |
| Race validation when supported | Yes or skipped | Yes or skipped | Yes or skipped | No |
| Optical hardware behavior | Manual | Manual | Manual | Yes |

The workflow prints the native Go version, operating system, architecture, and
`CGO_ENABLED` value. A skipped race or hardware check is reported as skipped,
not passed.

## Validation classes

Portable validation covers shared packages, the TUI, durable formats, catalog
logic, recovery scheduling, simulator scenarios, command safety, and bounded
runtime checks. Native validation compiles and tests platform-selected adapter
code on its matching runner. Hardware validation covers raw optical-device
access, permissions, media removal, stalled reads, and physical drive
behavior; GitHub-hosted runners do not provide that evidence.

## Local workflow

From the repository root:

```console
go run ./tools/devtool format
go run ./tools/devtool test
go run ./tools/devtool check
go run ./tools/devtool release --race=auto
```

Use `go run ./tools/devtool build --version ... --commit ... --build-date ...
--output ...` for metadata-aware builds. All subprocesses are invoked directly
with bounded argument lists; no platform shell is part of the project
contract.

## Hardware handoff

Do not treat a passing local or CI simulator run as proof of Linux, macOS, or
Windows optical-device behavior. Record the host OS, device model, permissions,
media state, and command-audit result for manual hardware validation. Device
work must remain read-only and must not be added to the normal CI gate.
