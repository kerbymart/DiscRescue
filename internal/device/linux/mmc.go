package linux

import (
	"encoding/binary"
	"fmt"

	"discrescue/internal/device"
)

type CDB []byte

func BuildCDB(kind device.CommandKind, request device.CommandRequest) (CDB, error) {
	if err := device.ValidateCommandKind(kind); err != nil {
		return nil, err
	}

	switch kind {
	case device.CommandInquiry:
		return CDB{0x12, 0x00, 0x00, 0x00, 0x24, 0x00}, nil
	case device.CommandTestReady:
		return CDB{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
	case device.CommandGetConfiguration:
		return CDB{0x46, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00}, nil
	case device.CommandReadCapacity:
		return CDB{0x25, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, nil
	case device.CommandReadTOC:
		return CDB{0x43, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x00, 0x00}, nil
	case device.CommandReadDiscInformation:
		return CDB{0x51, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x22, 0x00, 0x00}, nil
	case device.CommandReadDVDStructure:
		return CDB{0xad, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20}, nil
	case device.CommandReadBlocks:
		if request.Sectors == 0 {
			return nil, fmt.Errorf("build cdb: read blocks requires sectors")
		}
		if request.Sectors > 0xffff {
			return nil, fmt.Errorf("build cdb: read blocks sector count %d exceeds READ(10) limit", request.Sectors)
		}
		cdb := make(CDB, 10)
		cdb[0] = 0x28
		binary.BigEndian.PutUint32(cdb[2:6], uint32(request.StartLBA))
		binary.BigEndian.PutUint16(cdb[7:9], uint16(request.Sectors))
		return cdb, nil
	case device.CommandReadCD:
		if request.Sectors == 0 {
			return nil, fmt.Errorf("build cdb: read cd requires sectors")
		}
		if request.Sectors > 0xffffff {
			return nil, fmt.Errorf("build cdb: read cd sector count %d exceeds READ CD limit", request.Sectors)
		}
		cdb := make(CDB, 12)
		cdb[0] = 0xbe
		binary.BigEndian.PutUint32(cdb[2:6], uint32(request.StartLBA))
		cdb[6] = byte(request.Sectors >> 16)
		cdb[7] = byte(request.Sectors >> 8)
		cdb[8] = byte(request.Sectors)
		cdb[9] = 0x10
		return cdb, nil
	case device.CommandSetSpeed:
		if request.SpeedKbps == 0 {
			return nil, fmt.Errorf("build cdb: set speed requires a non-zero speed")
		}
		cdb := CDB{0xbb, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		binary.BigEndian.PutUint16(cdb[2:4], uint16(request.SpeedKbps))
		binary.BigEndian.PutUint16(cdb[4:6], uint16(request.SpeedKbps))
		return cdb, nil
	case device.CommandEject:
		return CDB{0x1b, 0x00, 0x00, 0x00, 0x02, 0x00}, nil
	default:
		return nil, fmt.Errorf("build cdb: unsupported command %q", kind)
	}
}
