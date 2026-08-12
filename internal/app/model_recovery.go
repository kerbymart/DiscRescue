package app

import "discrescue/internal/catalog"

type RecoveryViewModel struct {
	Phase              string
	ScannedSectors     uint64
	RecoveredSectors   uint64
	DeferredSectors    uint64
	TotalSectors       uint64
	UnreadableSectors  uint64
	Status             string
	OutputPath         string
	Elapsed            string
	Remaining          string
	ETA                string
	Throughput         string
	LastIssue          []string
	PausePending       bool
	StopPending        bool
	ForceStopAvailable bool
}

type ProgressSnapshot struct {
	Phase              string
	ScannedSectors     uint64
	RecoveredSectors   uint64
	DeferredSectors    uint64
	TotalSectors       uint64
	UnreadableSectors  uint64
	Status             string
	Elapsed            string
	Remaining          string
	ETA                string
	Throughput         string
	LastIssue          []string
	OutputPath         string
	PausePending       bool
	StopPending        bool
	ForceStopAvailable bool
}

type JobSummary struct {
	Outcome           string
	ImagePath         string
	MapPath           string
	NextAction        string
	UnresolvedSectors uint64
	DeferredSectors   uint64
	RecoveredSectors  uint64
	TotalSectors      uint64
	Duration          string
	CatalogStatus     catalog.CatalogWriteStatus
}
