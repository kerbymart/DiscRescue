package mapfile

type Checkpoint struct {
	LastSequence uint64
	Extents      []Extent
}
