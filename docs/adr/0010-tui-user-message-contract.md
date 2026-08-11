# ADR 0010: TUI user-message contract

## Status

Accepted

## Context

The TUI previously allowed individual workflow transitions and page renderers
to place arbitrary notices, including wrapped native errors, directly into the
primary status line. Empty states and failed operations could therefore repeat
the same condition in the title, card, and footer while using inconsistent
severity and wording.

## Decision

DiscRescue uses one shared `NoticeModel` as the primary user-message boundary.
It has a stable presentation code, severity, concise primary text, optional
explanation and next action, and a separate technical detail string.

Technical errors are translated in `internal/app/user_messages.go` using
project-owned error codes and typed classifications. The primary view never
renders the original native error string. When technical detail exists, the
user-facing notice offers `d` to open the details page, where the original
diagnostic is retained.

The shell status region owns transient notices. Each visible notice renders in
one semantic bordered container so its severity, message, and action remain a
single aligned unit. Page cards provide only context-specific instructions or
choices and must not restate the same status condition. Errors use the theme's
`Danger` role; warnings use the cohesive violet warning role rather than a
terminal-generic yellow/orange color; informational notices use the secondary
role. Text and markers remain understandable in monochrome mode.

## Consequences

- New user-facing operational failures must enter through `setErrorNotice` or
  an equivalent catalog-backed transition.
- Stable device/domain error codes can improve wording without matching native
  error strings.
- Technical diagnostics remain available without making the normal workflow
  depend on implementation terminology.
- Empty-state pages can keep their action controls while the shell owns the
  single primary condition message.
- At constrained terminal heights, the shell prioritizes the active page and
  action over a transient notice; the notice remains structured whenever it is
  shown.
