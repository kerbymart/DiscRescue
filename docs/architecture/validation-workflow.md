# DiscRescue Validation Workflow

This note separates Windows-local verification from future Linux hardware verification so task completion claims stay evidence-based.

## Verification matrix

| Verification area | Windows-local evidence | Linux-only evidence |
| --- | --- | --- |
| Formatting and package checks | `scripts/format.ps1`, `scripts/test.ps1`, `scripts/check.ps1` | rerun the same package checks on Linux before hardware release handoff |
| Bubble Tea workflow and layouts | `go test ./internal/app`, Windows Terminal manual checks | terminal restoration and layout behavior on the release shell and distro targets |
| Simulator recovery behavior | `internal/testdevice` scenarios and `scripts/release-gates.ps1` | rerun simulator and integration suites on Linux build agents |
| Device open and probe behavior | not authoritative on Windows | read-only optical-device probes against Linux block devices |
| `SG_IO` command behavior | fixed-vector tests only | fixed vectors plus Linux hardware traces allowed by public specs |
| Worker supervision under blocked I/O | simulator hung-worker coverage | Linux drive and bridge behavior under slow or stalled requests |
| Release race gate | only if `CGO_ENABLED=1` is set locally | required on Linux release validation when supported by the environment |

## Windows Local Checks

- `scripts/format.ps1`
- `scripts/test.ps1`
- `scripts/check.ps1`
- `go run ./cmd/discrescue`
- `scripts/release-gates.ps1`

For the TUI on Windows Terminal, include these evidence checks:

- `go test ./internal/app`
- verify `80x24`, `60x18`, and `40x12` layouts through the view tests;
- verify the below-minimum resize request at sizes smaller than `40x12`;
- verify monochrome-safe recovery rendering;
- verify long-path wrapping without horizontal scrolling;
- verify that recovery and details views preserve terminal state while resizing and restore correctly when leaving the alternate screen.

## Linux Hardware Checks

- Optical-device probe behavior against read-only opens
- `SG_IO` command and sense handling against fixed vectors and hardware traces permitted by public specs
- Worker supervision under slow or hung device operations
- Command-audit checks against the allowed non-destructive command set

## Release Gates

Release readiness uses `scripts/release-gates.ps1` as the single local entrypoint. That gate runs:

- formatting;
- baseline unit and package tests;
- vet and build;
- command-audit checks in `internal/testdevice`;
- simulator integration checks in `internal/testdevice`;
- soak and goroutine-leak checks in `internal/testdevice`;
- throughput and CPU benchmark commands for merge and verification paths;
- `go test -race ./...` only when `CGO_ENABLED=1`.

On the current Windows environment, a skipped race gate is not a passing race gate. The release handoff must report whether race coverage actually ran.

## Rule

Do not treat a passing Windows-local check as proof of Linux device behavior. Hardware-related tasks stay incomplete until their Linux-specific verification exists.
