package merge

type Provenance string

const (
	ProvenanceRead      Provenance = "read"
	ProvenanceVerified  Provenance = "verified"
	ProvenanceConflict  Provenance = "conflict"
	ProvenanceTentative Provenance = "tentative"
)
