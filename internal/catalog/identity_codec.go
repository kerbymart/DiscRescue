package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func marshalContentIdentity(identity ContentIdentity) ([]byte, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.LittleEndian, identity.Version)
	binary.Write(buf, binary.LittleEndian, identity.Profile)
	binary.Write(buf, binary.LittleEndian, identity.LogicalBlockSize)
	binary.Write(buf, binary.LittleEndian, identity.SectorCount)
	binary.Write(buf, binary.LittleEndian, identity.Sessions)
	binary.Write(buf, binary.LittleEndian, uint16(len(identity.Tracks)))
	for _, track := range identity.Tracks {
		binary.Write(buf, binary.LittleEndian, track.TrackNumber)
		binary.Write(buf, binary.LittleEndian, track.StartLBA)
		binary.Write(buf, binary.LittleEndian, track.EndLBA)
		binary.Write(buf, binary.LittleEndian, track.Mode)
		binary.Write(buf, binary.LittleEndian, track.ControlFlags)
		binary.Write(buf, binary.LittleEndian, track.LeadOutLBA)
	}
	if err := encodeString(buf, identity.LayoutSHA256); err != nil {
		return nil, err
	}
	binary.Write(buf, binary.LittleEndian, uint16(len(identity.VolumeHints)))
	for _, hint := range identity.VolumeHints {
		binary.Write(buf, binary.LittleEndian, hint.HintType)
		if err := encodeString(buf, hint.Value); err != nil {
			return nil, err
		}
	}
	binary.Write(buf, binary.LittleEndian, uint16(len(identity.Samples)))
	for _, sample := range identity.Samples {
		binary.Write(buf, binary.LittleEndian, sample.Slot)
		binary.Write(buf, binary.LittleEndian, sample.LBA)
		if err := encodeBool(buf, sample.Available); err != nil {
			return nil, err
		}
		buf.Write(sample.SHA256[:])
		binary.Write(buf, binary.LittleEndian, uint16(sample.Error))
	}
	if err := encodeString(buf, identity.QuickID); err != nil {
		return nil, err
	}
	if err := encodeString(buf, identity.FullContentSHA256); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func unmarshalContentIdentity(data []byte) (ContentIdentity, error) {
	offset := 0
	if len(data) < 2+2+4+8+2+2 {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: buffer too small")
	}
	identity := ContentIdentity{
		Version: binary.LittleEndian.Uint16(data[offset : offset+2]),
	}
	offset += 2
	identity.Profile = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	identity.LogicalBlockSize = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	identity.SectorCount = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	identity.Sessions = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	trackCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if trackCount > maxCatalogTracks || uint64(trackCount)*30 > uint64(len(data)-offset) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: track count %d exceeds bounds", trackCount)
	}
	identity.Tracks = make([]TrackLayout, 0, trackCount)
	for i := 0; i < trackCount; i++ {
		if offset+2+8+8+2+2+8 > len(data) {
			return ContentIdentity{}, fmt.Errorf("unmarshal content identity: truncated track")
		}
		identity.Tracks = append(identity.Tracks, TrackLayout{
			TrackNumber:  binary.LittleEndian.Uint16(data[offset : offset+2]),
			StartLBA:     int64(binary.LittleEndian.Uint64(data[offset+2 : offset+10])),
			EndLBA:       int64(binary.LittleEndian.Uint64(data[offset+10 : offset+18])),
			Mode:         binary.LittleEndian.Uint16(data[offset+18 : offset+20]),
			ControlFlags: binary.LittleEndian.Uint16(data[offset+20 : offset+22]),
			LeadOutLBA:   int64(binary.LittleEndian.Uint64(data[offset+22 : offset+30])),
		})
		offset += 30
	}
	var err error
	identity.LayoutSHA256, err = decodeString(data, &offset)
	if err != nil {
		return ContentIdentity{}, err
	}
	if offset+2 > len(data) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: truncated volume hint count")
	}
	hintCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if hintCount > maxCatalogHints || uint64(hintCount)*2 > uint64(len(data)-offset) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: volume hint count %d exceeds bounds", hintCount)
	}
	identity.VolumeHints = make([]VolumeHint, 0, hintCount)
	for i := 0; i < hintCount; i++ {
		if offset+2 > len(data) {
			return ContentIdentity{}, fmt.Errorf("unmarshal content identity: truncated volume hint type")
		}
		hint := VolumeHint{HintType: binary.LittleEndian.Uint16(data[offset : offset+2])}
		offset += 2
		hint.Value, err = decodeString(data, &offset)
		if err != nil {
			return ContentIdentity{}, err
		}
		identity.VolumeHints = append(identity.VolumeHints, hint)
	}
	if offset+2 > len(data) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: truncated sample count")
	}
	sampleCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if sampleCount > maxCatalogSamples || uint64(sampleCount)*45 > uint64(len(data)-offset) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: sample count %d exceeds bounds", sampleCount)
	}
	identity.Samples = make([]SectorFingerprint, 0, sampleCount)
	for i := 0; i < sampleCount; i++ {
		if offset+2+8+1+32+2 > len(data) {
			return ContentIdentity{}, fmt.Errorf("unmarshal content identity: truncated sample")
		}
		sample := SectorFingerprint{
			Slot: binary.LittleEndian.Uint16(data[offset : offset+2]),
			LBA:  int64(binary.LittleEndian.Uint64(data[offset+2 : offset+10])),
		}
		offset += 10
		sample.Available, err = decodeBool(data, &offset)
		if err != nil {
			return ContentIdentity{}, err
		}
		copy(sample.SHA256[:], data[offset:offset+32])
		offset += 32
		sample.Error = SampleErrorClass(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		identity.Samples = append(identity.Samples, sample)
	}
	identity.QuickID, err = decodeString(data, &offset)
	if err != nil {
		return ContentIdentity{}, err
	}
	identity.FullContentSHA256, err = decodeString(data, &offset)
	if err != nil {
		return ContentIdentity{}, err
	}
	if offset != len(data) {
		return ContentIdentity{}, fmt.Errorf("unmarshal content identity: trailing bytes %d", len(data)-offset)
	}
	return identity, identity.Validate()
}
