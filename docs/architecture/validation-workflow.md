# DiscRescue Validation Workflow

This note separates Windows-local verification from future Linux hardware verification so task completion claims stay evidence-based.

## Windows Local Checks

- `scripts/format.ps1`
- `scripts/test.ps1`
- `scripts/check.ps1`
- `go run ./cmd/discrescue`

## Linux Hardware Checks

- Optical-device probe behavior against read-only opens
- `SG_IO` command and sense handling against fixed vectors and hardware traces permitted by public specs
- Worker supervision under slow or hung device operations
- Command-audit checks against the allowed non-destructive command set

## Rule

Do not treat a passing Windows-local check as proof of Linux device behavior. Hardware-related tasks stay incomplete until their Linux-specific verification exists.
