package image

import "fmt"

type SectorWrite struct {
	LBA               uint64
	LogicalSectorSize uint32
	Data              []byte
}

func (w SectorWrite) Offset() uint64 {
	return w.LBA * uint64(w.LogicalSectorSize)
}

func (w SectorWrite) Validate() error {
	if uint32(len(w.Data)) != w.LogicalSectorSize {
		return fmt.Errorf("validate sector write: data length %d does not match sector size %d", len(w.Data), w.LogicalSectorSize)
	}
	return nil
}
