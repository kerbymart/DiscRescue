package mapfile

import "fmt"

type Extent struct {
	StartLBA uint64
	Sectors  uint32
	State    SectorState
}

func (e Extent) EndLBA() uint64 {
	return e.StartLBA + uint64(e.Sectors)
}

func (e Extent) Validate() error {
	if e.Sectors == 0 {
		return fmt.Errorf("validate extent: sectors must be greater than zero")
	}
	return nil
}

func Overlaps(left, right Extent) bool {
	return left.StartLBA < right.EndLBA() && right.StartLBA < left.EndLBA()
}
