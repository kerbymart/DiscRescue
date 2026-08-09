# Cross-Platform Development Contract

DiscRescue has one repository-owned development contract for macOS, Linux, and
Windows. The Go devtool is the canonical interface for formatting, tests,
checks, release validation, and metadata-aware builds.

## Requirements

- Go `1.26.5`, as declared by `go.mod`;
- a terminal capable of interactive keyboard input; and
- physical optical hardware only for hardware-specific validation.

The normal workflow does not require PowerShell, Bash, or another platform's
shell.

## Canonical commands

Run these commands from the repository root on every supported operating
system:

```console
go run ./tools/devtool format
go run ./tools/devtool test
go run ./tools/devtool check
go run ./tools/devtool build --output dist/discrescue
go run ./tools/devtool release --race=auto
```

`format --check` verifies without modifying files. `release --race=auto` runs
the race gate when the host supports it and reports an explicit `SKIPPED`
result otherwise. Use `--race=on` when race validation is mandatory or
`--race=off` when an intentional skip must be recorded.

The devtool invokes `gofmt` and Go directly through process arguments. It does
not construct shell command strings or invoke `sh`, Bash, PowerShell, `pwsh`,
or `cmd.exe`.

## Evidence boundaries

Portable checks run on all three CI hosts: formatting, vet, unit tests,
simulator scenarios, command audits, soak/leak checks, benchmarks, and builds.
Native adapter code is compiled on its matching operating system. Physical
optical-drive behavior is separate manual evidence and is never implied by a
passing GitHub-hosted runner.

## Platform-specific work

OS-specific device access belongs behind project-owned Go build constraints and
interfaces. Shared application code must not depend on native platform types.
Use the simulator for deterministic recovery behavior and use matching native
hardware only when validating optical-device behavior.

## TUI verification

On each supported terminal, verify keyboard-only navigation, terminal
restoration, and the `80x24`, `60x18`, `40x12`, and below-minimum layouts. These
checks complement the automated app tests; they do not require a particular
shell.
