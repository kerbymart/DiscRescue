# DiscRescue Validation Workflow

This note separates Windows-local verification from future Linux hardware verification so task completion claims stay evidence-based.

## Windows Local Checks

- `scripts/format.ps1`
- `scripts/test.ps1`
- `scripts/check.ps1`
- `go run ./cmd/discrescue`

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

## Rule

Do not treat a passing Windows-local check as proof of Linux device behavior. Hardware-related tasks stay incomplete until their Linux-specific verification exists.
