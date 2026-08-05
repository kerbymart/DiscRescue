package image

import "fmt"

type SectorWrite struct {
	LBA               uint64
	LogicalSectorSize uint32
	Data              []byte
}

type PositionedWrite struct {
	Offset uint64
	Data   []byte
}

type WriterPlan struct {
	LogicalSectorSize uint32
	ExpectedSectors   uint64
	Writes            []SectorWrite
}

func (w SectorWrite) Offset() uint64 {
	return w.LBA * uint64(w.LogicalSectorSize)
}

func (w SectorWrite) Validate() error {
	if w.LogicalSectorSize == 0 {
		return fmt.Errorf("validate sector write: logical sector size must be greater than zero")
	}
	if uint32(len(w.Data)) != w.LogicalSectorSize {
		return fmt.Errorf("validate sector write: data length %d does not match sector size %d", len(w.Data), w.LogicalSectorSize)
	}
	return nil
}

func (w SectorWrite) Positioned() (PositionedWrite, error) {
	if err := w.Validate(); err != nil {
		return PositionedWrite{}, err
	}
	return PositionedWrite{
		Offset: w.Offset(),
		Data:   append([]byte(nil), w.Data...),
	}, nil
}

func (p WriterPlan) Validate() error {
	if p.LogicalSectorSize == 0 {
		return fmt.Errorf("validate writer plan: logical sector size must be greater than zero")
	}
	for index, write := range p.Writes {
		if err := write.Validate(); err != nil {
			return fmt.Errorf("validate writer plan write[%d]: %w", index, err)
		}
		if write.LogicalSectorSize != p.LogicalSectorSize {
			return fmt.Errorf("validate writer plan write[%d]: sector size %d does not match plan size %d", index, write.LogicalSectorSize, p.LogicalSectorSize)
		}
		if p.ExpectedSectors > 0 && write.LBA >= p.ExpectedSectors {
			return fmt.Errorf("validate writer plan write[%d]: lba %d exceeds expected sector count %d", index, write.LBA, p.ExpectedSectors)
		}
		if index == 0 {
			continue
		}
		previous := p.Writes[index-1]
		if previous.LBA >= write.LBA {
			return fmt.Errorf("validate writer plan write[%d]: writes must be strictly ordered by lba", index)
		}
	}
	return nil
}

func BuildPositionedWrites(plan WriterPlan) ([]PositionedWrite, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if len(plan.Writes) == 0 {
		return nil, nil
	}

	positioned := make([]PositionedWrite, 0, len(plan.Writes))
	current := PositionedWrite{
		Offset: plan.Writes[0].Offset(),
		Data:   append([]byte(nil), plan.Writes[0].Data...),
	}

	for _, write := range plan.Writes[1:] {
		nextOffset := write.Offset()
		currentEnd := current.Offset + uint64(len(current.Data))
		if nextOffset == currentEnd {
			current.Data = append(current.Data, write.Data...)
			continue
		}

		positioned = append(positioned, current)
		current = PositionedWrite{
			Offset: nextOffset,
			Data:   append([]byte(nil), write.Data...),
		}
	}

	positioned = append(positioned, current)
	return positioned, nil
}
