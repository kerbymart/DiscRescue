package recoverymap

import (
	"encoding/hex"
	"fmt"

	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

// IdentityBinding copies the version-1 logical-content identity into the
// reserved recovery-map header fields. The map stores the first 16 bytes of
// the full QuickID digest as its compact source binding.
func IdentityBinding(header mapfile.Header, identity catalog.ContentIdentity, captureID, jobID, recordID catalog.RecordID) (mapfile.Header, error) {
	if err := identity.Validate(); err != nil {
		return mapfile.Header{}, fmt.Errorf("bind recovery map identity: %w", err)
	}
	layout, err := hex.DecodeString(identity.LayoutSHA256)
	if err != nil || len(layout) != len(header.LayoutSHA256) {
		return mapfile.Header{}, fmt.Errorf("bind recovery map identity: layout hash must be 32-byte hex")
	}
	header.IdentityAlgorithmVersion = identity.Version
	copy(header.LayoutSHA256[:], layout)
	header.QuickContentIDPresent = false
	header.QuickContentID = [16]byte{}
	if identity.QuickID != "" {
		quick, err := hex.DecodeString(identity.QuickID)
		if err != nil || len(quick) != 32 {
			return mapfile.Header{}, fmt.Errorf("bind recovery map identity: QuickID must be 32-byte hex")
		}
		header.QuickContentIDPresent = true
		copy(header.QuickContentID[:], quick[:len(header.QuickContentID)])
	}
	header.CaptureID = captureID
	header.JobID = jobID
	header.CatalogRecordIDPresent = recordID != (catalog.RecordID{})
	header.CatalogRecordID = recordID
	return header, nil
}

// ValidateIdentityBinding verifies the map's strong source binding against a
// newly collected identity. A map without a QuickID remains legacy or
// indeterminate and is never accepted as an automatic strong match.
func ValidateIdentityBinding(header mapfile.Header, observed catalog.ContentIdentity) error {
	if err := observed.Validate(); err != nil {
		return fmt.Errorf("validate recovery map identity: observed identity: %w", err)
	}
	if header.IdentityAlgorithmVersion == 0 || header.IdentityAlgorithmVersion != observed.Version {
		return fmt.Errorf("validate recovery map identity: algorithm version mismatch")
	}
	layout, err := hex.DecodeString(observed.LayoutSHA256)
	if err != nil || len(layout) != len(header.LayoutSHA256) || string(layout) != string(header.LayoutSHA256[:]) {
		return fmt.Errorf("validate recovery map identity: layout mismatch")
	}
	if !header.QuickContentIDPresent {
		return fmt.Errorf("validate recovery map identity: quick content binding is absent")
	}
	quick, err := hex.DecodeString(observed.QuickID)
	if err != nil || len(quick) != 32 {
		return fmt.Errorf("validate recovery map identity: observed QuickID is absent or invalid")
	}
	if string(quick[:len(header.QuickContentID)]) != string(header.QuickContentID[:]) {
		return fmt.Errorf("validate recovery map identity: quick content mismatch")
	}
	return nil
}
