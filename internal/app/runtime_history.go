package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"discrescue/internal/platform"
)

func (m ProgramModel) lookupPriorProcessing(basePath string) (PriorProcessingViewModel, []PriorProcessingRecord, []ResumableJobViewModel, error) {
	jobs, err := m.findResumableJobs(basePath)
	if err != nil {
		return PriorProcessingViewModel{}, nil, nil, err
	}
	if len(jobs) == 0 {
		return PriorProcessingViewModel{
			Kind:        PriorProcessingNone,
			HistoryLine: "No matching contents found on this computer.",
		}, nil, nil, nil
	}

	view := PriorProcessingViewModel{
		Kind:        PriorProcessingStrongResumable,
		Title:       "Matching contents were found on this computer",
		HistoryLine: fmt.Sprintf("Found %d resumable matching recoveries in %s.", len(jobs), strings.TrimSpace(basePath)),
		ImagePath:   jobs[0].OutputPath,
		Recovered:   formatCount(jobs[0].RecoveredSectors) + " sectors",
		Options: []string{
			"Resume the matching recovery",
			"Start a new recovery instead",
			"Choose another drive",
		},
	}
	if jobs[0].UnreadableSectors > 0 {
		view.UnreadableSectors = formatCount(jobs[0].UnreadableSectors) + " sectors"
	}
	view.Body = []string{
		"The current disc matches saved recovery work on this computer.",
		"Use Resume an unfinished recovery to continue from the saved map.",
	}
	records := make([]PriorProcessingRecord, 0, len(jobs))
	for _, job := range jobs {
		records = append(records, PriorProcessingRecord{
			Title:  filepath.Base(job.OutputPath),
			Detail: job.Detail,
		})
	}
	return view, records, jobs, nil
}
func (m ProgramModel) findResumableJobs(basePath string) ([]ResumableJobViewModel, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "."
	}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("find resumable recoveries in %s: %w", basePath, err)
	}

	jobs := make([]ResumableJobViewModel, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".iso") {
			continue
		}
		outputPath := filepath.Join(basePath, entry.Name())
		status, err := m.services.Recovery.InspectRecoveryTarget(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        outputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		if err != nil || !status.CanResume {
			continue
		}
		jobs = append(jobs, ResumableJobViewModel{
			OutputPath:        outputPath,
			MapPath:           status.MapPath,
			RecoveredSectors:  status.RecoveredSectors,
			UnreadableSectors: status.UnreadableSectors,
			Detail:            status.Detail,
		})
	}

	sort.Slice(jobs, func(i, j int) bool {
		return strings.ToLower(jobs[i].OutputPath) < strings.ToLower(jobs[j].OutputPath)
	})
	return jobs, nil
}
func (m ProgramModel) findProcessedMedia(basePath string) ([]ProcessedMediaViewModel, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = "."
	}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("browse processed media in %s: %w", basePath, err)
	}

	items := make([]ProcessedMediaViewModel, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".iso") {
			continue
		}
		outputPath := filepath.Join(basePath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect processed media %s: %w", outputPath, err)
		}
		mapPath := replaceExtension(outputPath, ".drmap")
		item := ProcessedMediaViewModel{
			Title:      entry.Name(),
			ImagePath:  outputPath,
			ModifiedAt: info.ModTime().Local().Format("2006-01-02 15:04"),
			Status:     "Image only",
			Detail:     "No recovery map was found next to this image.",
		}

		status, inspectErr := m.services.Recovery.InspectRecoveryTarget(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        outputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		switch {
		case inspectErr == nil && status.CanResume:
			item.MapPath = status.MapPath
			item.Status = "Resumable"
			item.Detail = status.Detail
		case fileExists(mapPath):
			item.MapPath = mapPath
			item.Status = "Saved with map"
			item.Detail = "A recovery map exists, but it does not match the currently selected disc."
		default:
			_ = status
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
	return items, nil
}
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
