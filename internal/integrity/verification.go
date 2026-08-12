package integrity

import (
	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

type ByteRange struct {
	Offset uint64
	Length uint64
}

type MapVerificationInput struct {
	HeaderBytes     []byte
	CheckpointBytes []byte
	JournalBytes    []byte
	ImageLength     uint64
}

type MapVerificationResult struct {
	Header             mapfile.Header
	ReplayedCheckpoint mapfile.Checkpoint
	RequiredImageBytes uint64
}

type ImageExtentInput struct {
	Extent mapfile.Extent
	Data   []byte
}

type ImageVerificationInput struct {
	LogicalSectorSize uint32
	Extents           []ImageExtentInput
}

type ImageVerificationResult struct {
	Extents       []mapfile.Extent
	ChangedRanges []ByteRange
	Provenance    []ExtentEvidence
}

type ExternalExtentInput struct {
	Extent  mapfile.Extent
	Data    []byte
	Digests []Digest
}

type ExternalVerificationInput struct {
	LogicalSectorSize uint32
	Extents           []ExternalExtentInput
}

type ExternalVerificationResult struct {
	Extents    []mapfile.Extent
	Provenance []ExtentEvidence
}

type CatalogRefreshInput struct {
	Entry             catalog.Entry
	FilesPresentByJob map[catalog.RecordID]bool
	State             catalog.ProcessingState
	FullContentSHA256 string
}

type FilesystemAdvisory struct {
	Code   string
	Detail string
}

type FilesystemAdvisoryResult struct {
	Advisories []FilesystemAdvisory
	Extents    []mapfile.Extent
}
