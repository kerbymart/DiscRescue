# ADR Template

Use this template for every architecture decision record added to `docs/adr/`.

## Filename

Use `NNNN-short-title.md`, where `NNNN` is a zero-padded sequence number.

## Structure

```md
# ADR NNNN: Short decision title

## Status

Accepted

## Date

YYYY-MM-DD

## Context

Describe the current problem, boundary, and why the decision is needed now.

## TDD Requirements

- Cite the exact TDD sections that constrain the decision.

## Decision

State the chosen approach and the invariant it preserves.

## Alternatives Considered

- Alternative 1
- Alternative 2

## Consequences

- What becomes simpler, safer, or more explicit.
- What remains intentionally out of scope.

## Migration or Rollback

Describe how to change course safely if later evidence requires it.

## Verification

- List the commands, tests, and documents that prove the decision is implemented.
```

## Rules

- Keep the decision scope narrow enough to review and verify in one change.
- Cite the relevant TDD sections directly.
- Record what remains intentionally out of scope.
- Update related format or architecture documents in the same change when the decision affects them.
