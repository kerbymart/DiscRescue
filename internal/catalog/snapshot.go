package catalog

import (
	"fmt"
	"hash/crc32"
)

const snapshotMagic = "DSCS"
const catalogFormatVersion uint16 = 1

const (
	maxCatalogEntries          = 100_000
	maxCatalogTracks           = 256
	maxCatalogHints            = 64
	maxCatalogSamples          = 64
	maxCatalogCapturesPerEntry = 1_024
	maxCatalogJobsPerEntry     = 1_024
)

var catalogCRC32CTable = crc32.MakeTable(crc32.Castagnoli)

type Snapshot struct {
	LastSequence uint64
	Entries      []Entry
}

func (s Snapshot) Validate() error {
	for i, entry := range s.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validate snapshot entry[%d]: %w", i, err)
		}
	}
	return nil
}
