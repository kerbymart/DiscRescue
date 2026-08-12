package mapfile

type Extent struct {
	StartLBA     uint64
	Sectors      uint32
	State        SectorState
	Confidence   Confidence
	Attempts     uint16
	CaptureID    uint32
	LastSenseKey uint8
	LastASC      uint8
	LastASCQ     uint8
	DataHash     [16]byte
}
