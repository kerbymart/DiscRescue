package catalog

import (
	"context"
	"fmt"
	"time"
)

// JobMetadata identifies one image/map pair and its capture provenance.
type JobMetadata struct {
	JobID     RecordID
	ImagePath string
	Capture   CaptureIdentity
}

// Service is the application-facing catalog boundary. It converts durable
// repository outcomes into typed write statuses and never changes recovery
// state when catalog persistence is unavailable.
type Service struct {
	repository *Repository
	now        func() time.Time
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

// RecordObserved creates a logical-content record after identity collection.
func (s *Service) RecordObserved(ctx context.Context, observation IdentityObservation, capture CaptureIdentity) (RecordID, CatalogWriteStatus) {
	if s == nil || s.repository == nil || !s.repository.mutable {
		return RecordID{}, CatalogWriteStatus{State: CatalogWriteUnavailable, Detail: "catalog is read-only or unavailable"}
	}
	if err := observation.Identity.Validate(); err != nil {
		return RecordID{}, failedCatalogWrite(err)
	}
	if err := capture.Validate(); err != nil {
		return RecordID{}, failedCatalogWrite(err)
	}
	recordID, err := NewRecordID()
	if err != nil {
		return RecordID{}, failedCatalogWrite(err)
	}
	now := s.now().UnixNano()
	entry := Entry{
		RecordID:          recordID,
		Identity:          observation.Identity,
		State:             ProcessingObserved,
		Status:            string(ProcessingObserved),
		FirstSeenUnixNano: now,
		LastSeenUnixNano:  now,
		Captures:          []CaptureIdentity{capture},
	}
	if err := s.repository.AppendEvent(ctx, CatalogEvent{Type: EventMediaObserved, Entry: entry}); err != nil {
		return RecordID{}, failedCatalogWrite(err)
	}
	return recordID, CatalogWriteStatus{State: CatalogWriteRecorded}
}

// RecordJobStarted links a durable image/map job to an existing content entry.
func (s *Service) RecordJobStarted(ctx context.Context, recordID RecordID, metadata JobMetadata) CatalogWriteStatus {
	return s.recordJobState(ctx, recordID, metadata, ProcessingInProgress)
}

// RecordJobState records a lifecycle state after recovery artifacts have been
// durably finalized or durably checkpointed.
func (s *Service) RecordJobState(ctx context.Context, recordID RecordID, metadata JobMetadata, state ProcessingState) CatalogWriteStatus {
	return s.recordJobState(ctx, recordID, metadata, state)
}

func (s *Service) recordJobState(ctx context.Context, recordID RecordID, metadata JobMetadata, state ProcessingState) CatalogWriteStatus {
	if s == nil || s.repository == nil || !s.repository.mutable {
		return CatalogWriteStatus{State: CatalogWriteUnavailable, Detail: "catalog is read-only or unavailable"}
	}
	if metadata.JobID == (RecordID{}) || metadata.ImagePath == "" || !validProcessingState(state) {
		return failedCatalogWrite(fmt.Errorf("invalid job metadata or processing state"))
	}
	entry, found := findEntry(s.repository.store, recordID)
	if !found {
		return failedCatalogWrite(fmt.Errorf("catalog record %x not found", recordID))
	}
	entry.State = state
	entry.Status = string(state)
	entry.LastSeenUnixNano = s.now().UnixNano()
	jobReference := JobReference{JobID: metadata.JobID, Path: metadata.ImagePath}
	replacedJob := false
	for i := range entry.JobReferences {
		if entry.JobReferences[i].JobID == metadata.JobID {
			entry.JobReferences[i] = jobReference
			replacedJob = true
			break
		}
	}
	if !replacedJob {
		entry.JobReferences = append(entry.JobReferences, jobReference)
	}
	entry.PreferredJobID = metadata.JobID
	if metadata.Capture.CaptureID != "" {
		replacedCapture := false
		for i := range entry.Captures {
			if entry.Captures[i].CaptureID == metadata.Capture.CaptureID {
				entry.Captures[i] = metadata.Capture
				replacedCapture = true
				break
			}
		}
		if !replacedCapture {
			entry.Captures = append(entry.Captures, metadata.Capture)
		}
	}
	if err := s.repository.AppendEvent(ctx, CatalogEvent{Type: EventJobStateChanged, Entry: entry}); err != nil {
		return failedCatalogWrite(err)
	}
	return CatalogWriteStatus{State: CatalogWriteRecorded}
}

func findEntry(store Store, recordID RecordID) (Entry, bool) {
	for _, entry := range store.Entries {
		if entry.RecordID == recordID {
			return entry, true
		}
	}
	return Entry{}, false
}

func failedCatalogWrite(err error) CatalogWriteStatus {
	return CatalogWriteStatus{State: CatalogWriteFailed, Detail: err.Error()}
}
