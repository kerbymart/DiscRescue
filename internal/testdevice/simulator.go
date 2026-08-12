package testdevice

import "fmt"

type Range struct {
	StartLBA uint64
	Sectors  uint32
}

func (r Range) EndLBA() uint64 {
	return r.StartLBA + uint64(r.Sectors)
}

func (r Range) Validate() error {
	if r.Sectors == 0 {
		return fmt.Errorf("validate range: sectors must be greater than zero")
	}
	return nil
}

func Overlap(left, right Range) bool {
	return left.StartLBA < right.EndLBA() && right.StartLBA < left.EndLBA()
}

type SenseTuple struct {
	Key  uint8
	ASC  uint8
	ASCQ uint8
}

type PassName string

const (
	PassAny      PassName = "any"
	PassFast     PassName = "fast"
	PassTrim     PassName = "trim"
	PassAdaptive PassName = "adaptive"
	PassScrape   PassName = "scrape"
	PassVerify   PassName = "verify"
)

type AttemptSelector struct {
	Pass       PassName
	AttemptMin uint16
	AttemptMax uint16
}

func (s AttemptSelector) Validate() error {
	if s.AttemptMax > 0 && s.AttemptMax < s.AttemptMin {
		return fmt.Errorf("validate attempt selector: max attempt %d is smaller than min attempt %d", s.AttemptMax, s.AttemptMin)
	}
	return nil
}
