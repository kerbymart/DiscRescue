# DiscRescue Validation Workflow

The repository-owned Go devtool is the canonical MVP validation entrypoint on
Linux, macOS, and Windows. GitHub-hosted automation is deferred infrastructure;
it is not a runtime dependency or a blocker for private MVP development.

## Canonical local commands

Run these from the repository root on each available supported operating system:

```console
go run ./tools/devtool format --check
go run ./tools/devtool test
go run ./tools/devtool check
go run ./tools/devtool release --race=auto
```

The devtool runs commands directly with `os/exec.CommandContext`; it does not
require PowerShell, Bash, `cmd.exe`, or another shell wrapper.

## Evidence matrix

| Verification area | Portable/native local evidence | Hardware evidence |
| --- | --- | --- |
| Formatting and package checks | `go run ./tools/devtool check` on each OS | none |
| Bubble Tea workflow and layouts | `go test ./internal/app` plus native terminal checks | terminal restoration on target release shells |
| Simulator recovery behavior | `go run ./tools/devtool release --race=auto` | none; simulator evidence is portable |
| Device open and probe behavior | platform-specific Go builds and tests | read-only optical-device probes on the target OS |
| `SG_IO` command behavior | fixed-vector tests | Linux traces permitted by public specifications |
| Worker supervision under blocked I/O | simulator hung-worker coverage | drive and bridge behavior under slow or stalled requests |
| Race validation | native run when supported; otherwise explicitly `SKIPPED` | rerun on release target when required |

Cross-compilation supplements native evidence but does not replace it.

## Release gate contents

`go run ./tools/devtool release --race=auto` runs:

- format verification;
- baseline tests;
- vet and trimmed build;
- command-audit tests;
- simulator integration tests;
- soak and goroutine-leak validation;
- race tests when supported;
- throughput and CPU benchmark smoke tests.

An unavailable race gate is not a passing race gate. The release handoff must
report whether race coverage actually ran.

## Hardware validation

Hardware-related tasks stay incomplete until target-OS evidence exists. Do not
infer optical-device behavior from simulator tests or cross-compilation alone.

- Linux: read-only optical-device probes, `SG_IO` behavior, and worker behavior
  against real hardware where available.
- macOS: read-only raw-device access and `diskutil` discovery behavior.
- Windows: native platform adapter and optical-volume behavior.

## Deferred automation

Issue #6 tracks the future GitHub Actions three-OS matrix. It remains open while
hosted runners are unavailable. When enabled, it should invoke this same
`go run ./tools/devtool release` command on native Linux, macOS, and Windows
runners. GitHub Actions is release-engineering automation, not a DiscRescue
runtime dependency.
