package integrity

import (
	"fmt"

	"discrescue/internal/mapfile"
)

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
