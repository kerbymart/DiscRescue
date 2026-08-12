package mapfile

import (
	"fmt"
	"hash/crc32"
)

const headerLength = 135

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type Header struct {
	LogicalSectorSize        uint32
	ExpectedSectorCount      uint64
	OutputFormat             uint16
	IdentityAlgorithmVersion uint16
	LayoutSHA256             [32]byte
	QuickContentIDPresent    bool
	QuickContentID           [16]byte
	CaptureID                [16]byte
	CatalogRecordIDPresent   bool
	CatalogRecordID          [16]byte
	JobID                    [16]byte
	CreationUnixNano         int64
	CleanShutdown            bool
}

func (h Header) Validate() error {
	if h.LogicalSectorSize == 0 {
		return fmt.Errorf("validate header: logical sector size must be greater than zero")
	}
	if h.ExpectedSectorCount == 0 {
		return fmt.Errorf("validate header: expected sector count must be greater than zero")
	}
	return nil
}
