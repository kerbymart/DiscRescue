package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func marshalCaptureIdentity(capture CaptureIdentity) ([]byte, error) {
	if err := capture.Validate(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	if err := encodeString(buf, capture.CaptureID); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.Device.Vendor); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.Device.Product); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.Device.Revision); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.Device.Serial); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.Device.Transport); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, capture.StartedAt.UnixNano()); err != nil {
		return nil, err
	}
	if err := encodeString(buf, capture.UserLabel); err != nil {
		return nil, err
	}
	if err := encodeBool(buf, capture.PhysicalCopy != nil); err != nil {
		return nil, err
	}
	if capture.PhysicalCopy != nil {
		if err := encodeString(buf, capture.PhysicalCopy.AssetID); err != nil {
			return nil, err
		}
		if err := encodeString(buf, capture.PhysicalCopy.HubCodeNote); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
func unmarshalCaptureIdentity(data []byte) (CaptureIdentity, error) {
	offset := 0
	var err error
	var capture CaptureIdentity
	capture.CaptureID, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	capture.Device.Vendor, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	capture.Device.Product, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	capture.Device.Revision, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	capture.Device.Serial, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	capture.Device.Transport, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	if offset+8 > len(data) {
		return CaptureIdentity{}, fmt.Errorf("unmarshal capture identity: truncated started-at")
	}
	capture.StartedAt = unixNanoUTC(int64(binary.LittleEndian.Uint64(data[offset : offset+8])))
	offset += 8
	capture.UserLabel, err = decodeString(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	hasPhysicalCopy, err := decodeBool(data, &offset)
	if err != nil {
		return CaptureIdentity{}, err
	}
	if hasPhysicalCopy {
		physical := &PhysicalCopyIdentity{}
		physical.AssetID, err = decodeString(data, &offset)
		if err != nil {
			return CaptureIdentity{}, err
		}
		physical.HubCodeNote, err = decodeString(data, &offset)
		if err != nil {
			return CaptureIdentity{}, err
		}
		capture.PhysicalCopy = physical
	}
	if offset != len(data) {
		return CaptureIdentity{}, fmt.Errorf("unmarshal capture identity: trailing bytes %d", len(data)-offset)
	}
	return capture, capture.Validate()
}
