package device

import "fmt"

type SenseClass string

const (
	SenseRetryable SenseClass = "retryable"
	SenseFatal     SenseClass = "fatal"
)

type SenseTuple struct {
	Key  uint8
	ASC  uint8
	ASCQ uint8
}

func ParseFixedFormatSense(data []byte) (SenseTuple, error) {
	if len(data) < 14 {
		return SenseTuple{}, fmt.Errorf("parse fixed format sense: expected at least 14 bytes, got %d", len(data))
	}
	return SenseTuple{
		Key:  data[2] & 0x0f,
		ASC:  data[12],
		ASCQ: data[13],
	}, nil
}

func ClassifySense(tuple SenseTuple) SenseClass {
	switch tuple.Key {
	case 0x02, 0x06:
		return SenseRetryable
	case 0x03, 0x05, 0x07:
		return SenseFatal
	default:
		if tuple.ASC == 0x04 && tuple.ASCQ == 0x01 {
			return SenseRetryable
		}
		return SenseFatal
	}
}
