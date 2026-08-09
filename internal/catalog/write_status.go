package catalog

import "fmt"

// CatalogWriteState describes the outcome of one catalog persistence attempt.
// A successful recovery and a successful catalog write are separate outcomes.
type CatalogWriteState string

const (
	CatalogWriteNotAttempted CatalogWriteState = "not_attempted"
	CatalogWriteRecorded     CatalogWriteState = "recorded"
	CatalogWriteUnavailable  CatalogWriteState = "unavailable"
	CatalogWriteFailed       CatalogWriteState = "failed"
)

// CatalogWriteStatus is the typed, user-visible result of a catalog write.
type CatalogWriteStatus struct {
	State  CatalogWriteState
	Detail string
}

// Validate rejects statuses that cannot be rendered or persisted safely.
func (s CatalogWriteStatus) Validate() error {
	switch s.State {
	case CatalogWriteNotAttempted, CatalogWriteRecorded, CatalogWriteUnavailable, CatalogWriteFailed:
		return nil
	default:
		return fmt.Errorf("validate catalog write status: unsupported state %q", s.State)
	}
}

// SummaryLine returns the optional history line for the recovery summary.
// Empty means the summary should omit the history line.
func (s CatalogWriteStatus) SummaryLine() string {
	switch s.State {
	case CatalogWriteRecorded:
		return "Recorded in local processed-media catalog"
	case CatalogWriteUnavailable:
		return "Catalog unavailable; recovery files are still saved"
	case CatalogWriteFailed:
		return "Catalog update failed"
	default:
		return ""
	}
}
