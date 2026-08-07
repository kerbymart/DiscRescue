#!/usr/bin/env bash
set -euo pipefail
log=${1:-discrescue-debug.log}
if [[ ! -f "$log" ]]; then
  printf 'Log not found: %s\n' "$log" >&2
  exit 1
fi
if command -v gum >/dev/null 2>&1; then
  gum pager < "$log"
else
  printf 'Gum is not installed; inspect the log directly with: less -- %q\n' "$log" >&2
  less -- "$log"
fi
