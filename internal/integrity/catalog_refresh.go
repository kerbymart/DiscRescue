package integrity

import (
	"fmt"

	"discrescue/internal/catalog"
)

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
