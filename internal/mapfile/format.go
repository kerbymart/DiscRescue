package mapfile

const HeaderMagic = "DSR1"
const FormatVersion uint16 = 1

type SectorState string

const (
	SectorStateUnknown   SectorState = "unknown"
	SectorStateQueued    SectorState = "queued"
	SectorStateRecovered SectorState = "recovered"
	SectorStateMissing   SectorState = "missing"
)

type Confidence string

const (
	ConfidenceTransport Confidence = "transport"
	ConfidenceVerified  Confidence = "verified"
	ConfidenceConflict  Confidence = "conflict"
)

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
