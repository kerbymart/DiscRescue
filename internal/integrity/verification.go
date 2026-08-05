package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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

func VerifyMap(input MapVerificationInput) (MapVerificationResult, error) {
	header, err := mapfile.UnmarshalHeader(input.HeaderBytes)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map header: %w", err)
	}

	checkpoint, err := mapfile.UnmarshalCheckpoint(input.CheckpointBytes)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map checkpoint: %w", err)
	}

	replayed, err := mapfile.ReplayJournal(checkpoint, input.JournalBytes)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map journal: %w", err)
	}

	requiredImageBytes := requiredImageBytes(replayed.Extents, header.LogicalSectorSize)
	if input.ImageLength < requiredImageBytes {
		return MapVerificationResult{}, fmt.Errorf(
			"verify map image length: image length %d is smaller than required %d",
			input.ImageLength,
			requiredImageBytes,
		)
	}

	return MapVerificationResult{
		Header:             header,
		ReplayedCheckpoint: replayed,
		RequiredImageBytes: requiredImageBytes,
	}, nil
}

func VerifyImage(input ImageVerificationInput) (ImageVerificationResult, error) {
	if input.LogicalSectorSize == 0 {
		return ImageVerificationResult{}, fmt.Errorf("verify image: logical sector size must be greater than zero")
	}

	result := ImageVerificationResult{
		Extents:    make([]mapfile.Extent, 0, len(input.Extents)),
		Provenance: make([]ExtentEvidence, 0, len(input.Extents)),
	}

	for i, item := range input.Extents {
		if err := item.Extent.Validate(); err != nil {
			return ImageVerificationResult{}, fmt.Errorf("verify image extent[%d]: %w", i, err)
		}
		if !claimsImageData(item.Extent.State) {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		expectedLength := uint64(item.Extent.Sectors) * uint64(input.LogicalSectorSize)
		if uint64(len(item.Data)) != expectedLength {
			return ImageVerificationResult{}, fmt.Errorf(
				"verify image extent[%d]: data length %d does not match expected %d",
				i,
				len(item.Data),
				expectedLength,
			)
		}

		if item.Extent.DataHash == ([16]byte{}) {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		if hash16(item.Data) == item.Extent.DataHash {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		downgraded := item.Extent
		downgraded.State = mapfile.SectorStateChecksumError
		downgraded.Confidence = mapfile.ConfidenceNone
		result.Extents = append(result.Extents, downgraded)
		result.Provenance = append(result.Provenance, ExtentEvidence{
			StartLBA: downgraded.StartLBA,
			Sectors:  downgraded.Sectors,
			Items:    nil,
		})
		result.ChangedRanges = append(result.ChangedRanges, ByteRange{
			Offset: item.Extent.StartLBA * uint64(input.LogicalSectorSize),
			Length: expectedLength,
		})
	}

	return result, nil
}

func VerifyExternal(input ExternalVerificationInput) (ExternalVerificationResult, error) {
	if input.LogicalSectorSize == 0 {
		return ExternalVerificationResult{}, fmt.Errorf("verify external: logical sector size must be greater than zero")
	}

	result := ExternalVerificationResult{
		Extents:    make([]mapfile.Extent, 0, len(input.Extents)),
		Provenance: make([]ExtentEvidence, 0, len(input.Extents)),
	}

	for i, item := range input.Extents {
		if err := item.Extent.Validate(); err != nil {
			return ExternalVerificationResult{}, fmt.Errorf("verify external extent[%d]: %w", i, err)
		}
		if !claimsImageData(item.Extent.State) {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		expectedLength := uint64(item.Extent.Sectors) * uint64(input.LogicalSectorSize)
		if uint64(len(item.Data)) != expectedLength {
			return ExternalVerificationResult{}, fmt.Errorf(
				"verify external extent[%d]: data length %d does not match expected %d",
				i,
				len(item.Data),
				expectedLength,
			)
		}

		verified := false
		for _, digest := range item.Digests {
			ok, err := verifyDigest(item.Data, digest)
			if err != nil {
				return ExternalVerificationResult{}, fmt.Errorf("verify external extent[%d]: %w", i, err)
			}
			if !ok {
				continue
			}

			next := item.Extent
			if item.Extent.State == mapfile.SectorStateReconstructed {
				next.Confidence = mapfile.ConfidenceReconstructedChecksum
			} else {
				next.State = mapfile.SectorStateVerified
				next.Confidence = mapfile.ConfidenceTrustedChecksum
			}
			result.Extents = append(result.Extents, next)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: next.StartLBA,
				Sectors:  next.Sectors,
				Items:    EvidenceFromStateConfidence(next),
			})
			verified = true
			break
		}

		if verified {
			continue
		}

		downgraded := item.Extent
		downgraded.State = mapfile.SectorStateChecksumError
		downgraded.Confidence = mapfile.ConfidenceNone
		result.Extents = append(result.Extents, downgraded)
		result.Provenance = append(result.Provenance, ExtentEvidence{
			StartLBA: downgraded.StartLBA,
			Sectors:  downgraded.Sectors,
			Items:    nil,
		})
	}

	return result, nil
}

func RefreshCatalogEntry(input CatalogRefreshInput) (catalog.Entry, error) {
	entry := input.Entry
	if err := entry.Validate(); err != nil {
		return catalog.Entry{}, fmt.Errorf("refresh catalog entry: %w", err)
	}

	for i := range entry.JobReferences {
		if present, ok := input.FilesPresentByJob[entry.JobReferences[i].JobID]; ok {
			entry.JobReferences[i].FilesPresent = present
		}
	}

	entry.State = input.State
	if input.FullContentSHA256 != "" {
		entry.Identity.FullContentSHA256 = input.FullContentSHA256
	}

	if err := entry.Validate(); err != nil {
		return catalog.Entry{}, fmt.Errorf("refresh catalog entry result: %w", err)
	}
	return entry, nil
}

func EvaluateFilesystemAdvisories(advisories []FilesystemAdvisory, extents []mapfile.Extent) (FilesystemAdvisoryResult, error) {
	for i, advisory := range advisories {
		if advisory.Code == "" {
			return FilesystemAdvisoryResult{}, fmt.Errorf("filesystem advisory[%d]: code is required", i)
		}
		if advisory.Detail == "" {
			return FilesystemAdvisoryResult{}, fmt.Errorf("filesystem advisory[%d]: detail is required", i)
		}
	}
	if err := mapfile.ValidateExtentSet(extents); err != nil {
		return FilesystemAdvisoryResult{}, fmt.Errorf("filesystem advisories extents: %w", err)
	}
	return FilesystemAdvisoryResult{
		Advisories: append([]FilesystemAdvisory(nil), advisories...),
		Extents:    append([]mapfile.Extent(nil), extents...),
	}, nil
}

func requiredImageBytes(extents []mapfile.Extent, logicalSectorSize uint32) uint64 {
	var maxEnd uint64
	for _, extent := range extents {
		if !claimsImageData(extent.State) {
			continue
		}
		if extent.EndLBA() > maxEnd {
			maxEnd = extent.EndLBA()
		}
	}
	return maxEnd * uint64(logicalSectorSize)
}

func claimsImageData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified,
		mapfile.SectorStateVerified,
		mapfile.SectorStateChecksumError,
		mapfile.SectorStateConflicting,
		mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}

func hash16(data []byte) [16]byte {
	sum := sha256.Sum256(data)
	var out [16]byte
	copy(out[:], sum[:16])
	return out
}

func verifyDigest(data []byte, digest Digest) (bool, error) {
	switch digest.Provider {
	case ProviderSHA256:
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]) == strings.ToLower(digest.Value), nil
	case ProviderNone:
		return false, fmt.Errorf("verify digest: provider none is not verifiable")
	default:
		return false, fmt.Errorf("verify digest: unsupported provider %q", digest.Provider)
	}
}
