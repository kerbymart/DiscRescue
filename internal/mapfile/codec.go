package mapfile

import (
	"encoding/binary"
	"fmt"
)

const extentEncodedBytes = 39

func MarshalExtent(extent Extent) ([]byte, error) {
	if err := extent.Validate(); err != nil {
		return nil, err
	}

	encoded := make([]byte, extentEncodedBytes)
	binary.LittleEndian.PutUint64(encoded[0:8], extent.StartLBA)
	binary.LittleEndian.PutUint32(encoded[8:12], extent.Sectors)
	encoded[12] = byte(extent.State)
	encoded[13] = byte(extent.Confidence)
	binary.LittleEndian.PutUint16(encoded[14:16], extent.Attempts)
	binary.LittleEndian.PutUint32(encoded[16:20], extent.CaptureID)
	encoded[20] = extent.LastSenseKey
	encoded[21] = extent.LastASC
	encoded[22] = extent.LastASCQ
	copy(encoded[23:39], extent.DataHash[:])
	return encoded, nil
}

func UnmarshalExtent(encoded []byte) (Extent, error) {
	if len(encoded) != extentEncodedBytes {
		return Extent{}, fmt.Errorf("unmarshal extent: expected %d bytes, got %d", extentEncodedBytes, len(encoded))
	}

	var extent Extent
	extent.StartLBA = binary.LittleEndian.Uint64(encoded[0:8])
	extent.Sectors = binary.LittleEndian.Uint32(encoded[8:12])
	extent.State = SectorState(encoded[12])
	extent.Confidence = Confidence(encoded[13])
	extent.Attempts = binary.LittleEndian.Uint16(encoded[14:16])
	extent.CaptureID = binary.LittleEndian.Uint32(encoded[16:20])
	extent.LastSenseKey = encoded[20]
	extent.LastASC = encoded[21]
	extent.LastASCQ = encoded[22]
	copy(extent.DataHash[:], encoded[23:39])
	return extent, extent.Validate()
}
