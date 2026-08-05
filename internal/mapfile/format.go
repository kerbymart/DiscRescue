package mapfile

const HeaderMagic = "DSR1"
const FormatVersion uint16 = 1

type SectorState uint8

const (
	SectorStateUnknown SectorState = iota
	SectorStateQueued
	SectorStateReadUnverified
	SectorStateVerified
	SectorStateMissing
	SectorStateIOError
	SectorStateChecksumError
	SectorStateConflicting
	SectorStateReconstructed
	SectorStateSkipped
)

func (s SectorState) String() string {
	switch s {
	case SectorStateUnknown:
		return "unknown"
	case SectorStateQueued:
		return "queued"
	case SectorStateReadUnverified:
		return "read_unverified"
	case SectorStateVerified:
		return "verified"
	case SectorStateMissing:
		return "missing"
	case SectorStateIOError:
		return "io_error"
	case SectorStateChecksumError:
		return "checksum_error"
	case SectorStateConflicting:
		return "conflicting"
	case SectorStateReconstructed:
		return "reconstructed"
	case SectorStateSkipped:
		return "skipped"
	default:
		return "invalid"
	}
}

type Confidence uint8

const (
	ConfidenceNone Confidence = iota
	ConfidenceSingleRead
	ConfidenceRepeatedSingleCapture
	ConfidenceRepeatedIndependentCapture
	ConfidenceTrustedChecksum
	ConfidenceReconstructedChecksum
)

func (c Confidence) String() string {
	switch c {
	case ConfidenceNone:
		return "none"
	case ConfidenceSingleRead:
		return "single_read"
	case ConfidenceRepeatedSingleCapture:
		return "repeated_single_capture"
	case ConfidenceRepeatedIndependentCapture:
		return "repeated_independent_capture"
	case ConfidenceTrustedChecksum:
		return "trusted_checksum"
	case ConfidenceReconstructedChecksum:
		return "reconstructed_checksum"
	default:
		return "invalid"
	}
}

type RecordType string

const (
	RecordJobCreated          RecordType = "JOB_CREATED"
	RecordCaptureOpened       RecordType = "CAPTURE_OPENED"
	RecordPassStarted         RecordType = "PASS_STARTED"
	RecordDataWritten         RecordType = "DATA_WRITTEN"
	RecordExtentStateChanged  RecordType = "EXTENT_STATE_CHANGED"
	RecordErrorRecorded       RecordType = "ERROR_RECORDED"
	RecordMediaReidentified   RecordType = "MEDIA_REIDENTIFIED"
	RecordCheckpointCommitted RecordType = "CHECKPOINT_COMMITTED"
	RecordPassFinished        RecordType = "PASS_FINISHED"
	RecordJobStopped          RecordType = "JOB_STOPPED"
	RecordJobCompleted        RecordType = "JOB_COMPLETED"
)
