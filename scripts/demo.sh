#!/usr/bin/env bash
set -euo pipefail

if ! command -v gum >/dev/null 2>&1; then
  printf '%s\n' 'Gum is not installed; run the underlying command directly:' '  go run ./cmd/discrescue' >&2
  exit 1
fi

gum style --border rounded --padding '0 1' --border-foreground 62 'DiscRescue demo' 'safe, deterministic TUI preview'
scenario="$(gum choose --header 'Choose a controlled demo scenario' \
  'healthy recovery' 'deferred sectors' 'unreadable sectors' 'paused recovery' 'resumed recovery')" || exit 0

gum spin --spinner dot --title "Building DiscRescue for ${scenario}" -- go build -trimpath -o ./discrescue ./cmd/discrescue
export DISKRESCUE_DISCOVERY_DRIVES='demo|Synthetic optical drive|available'
./discrescue

if [[ -f discrescue-debug.log ]] && gum confirm 'Inspect the captured debug log?'; then
  gum pager < discrescue-debug.log
fi

if gum confirm 'Remove the generated demo executable and artifacts?'; then
  rm -f -- ./discrescue ./discrescue-debug.log
fi
