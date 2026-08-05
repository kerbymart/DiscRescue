package mapfile

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
