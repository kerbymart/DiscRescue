# Device refresh and media reconciliation

The drive chooser exposes `r` as an explicit refresh action. Refresh results
carry a request generation and are ignored when their request ID is stale.
Reconciliation uses the stable device ID when one is available and falls back
to the path only for path-based adapters. A path change for the same device
preserves the selection; a removed device clears it.

Media state is invalidated when the selected device disappears or its media
token changes. The application must inspect the current media again before
resume/history decisions are available. Refresh is a chooser action and is not
started while an active recovery owns the drive.
