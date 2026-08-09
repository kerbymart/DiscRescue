# Cross-Platform Development Contract

DiscRescue supports Linux, macOS, and Windows development. The repository-owned
Go devtool is the shell-neutral interface for formatting, testing, checking,
release validation, and metadata-aware builds.

## Requirements

- Go 1.26.5 or newer.
- A terminal capable of interactive keyboard input for the TUI.
- No PowerShell, Bash, `cmd.exe`, or other shell is required for repository
  validation.

## Canonical commands

From the repository root:

```console
go run ./tools/devtool format
go run ./tools/devtool format --check
go run ./tools/devtool test
go run ./tools/devtool check
go run ./tools/devtool release --race=auto
go run ./tools/devtool build --version 0.1.0 --commit dev --build-date 2026-08-10 --output dist/discrescue
```

The devtool invokes Go programs directly with `os/exec.CommandContext`; it does
not construct shell command strings. Paths containing spaces are passed as
individual arguments.

## Evidence levels

- Portable: shared packages, TUI tests, map/catalog formats, simulator tests,
  command audits, and recovery planning.
- Native: platform-selected Go packages, native builds, terminal behavior, and
  race validation when supported by the local toolchain.
- Hardware: read-only optical-device probes and public-spec protocol behavior
  against real target hardware. Hardware claims are never inferred from
  simulator or cross-compilation success.

Cross-compilation is useful as a supplement. It is not native validation.

## Automation status

GitHub Actions is deferred release-engineering infrastructure for the private
MVP. Issue #6 remains open until hosted runners are available, and its future
three-OS workflow must consume the same `go run ./tools/devtool release` command.
