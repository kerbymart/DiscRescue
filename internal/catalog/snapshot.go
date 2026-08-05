package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const snapshotMagic = "DSCS"
const catalogFormatVersion uint16 = 1

var catalogCRC32CTable = crc32.MakeTable(crc32.Castagnoli)

type Snapshot struct {
	LastSequence uint64
	Entries      []Entry
}

func (s Snapshot) Validate() error {
	for i, entry := range s.Entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validate snapshot entry[%d]: %w", i, err)
		}
	}
	return nil
}

func MarshalSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	payload, err := marshalEntries(snapshot.Entries)
	if err != nil {
		return nil, err
	}

	encoded := make([]byte, 4+2+4+8+4+len(payload)+4)
	copy(encoded[0:4], []byte(snapshotMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], catalogFormatVersion)
	binary.LittleEndian.PutUint32(encoded[6:10], uint32(len(payload)))
	binary.LittleEndian.PutUint64(encoded[10:18], snapshot.LastSequence)
	binary.LittleEndian.PutUint32(encoded[18:22], uint32(len(snapshot.Entries)))
	copy(encoded[22:22+len(payload)], payload)
	crc := crc32.Checksum(encoded[:len(encoded)-4], catalogCRC32CTable)
	binary.LittleEndian.PutUint32(encoded[len(encoded)-4:], crc)
	return encoded, nil
}

func UnmarshalSnapshot(encoded []byte) (Snapshot, error) {
	if len(encoded) < 26 {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: buffer too small")
	}
	if string(encoded[0:4]) != snapshotMagic {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != catalogFormatVersion {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: unsupported version %d", version)
	}
	payloadLength := int(binary.LittleEndian.Uint32(encoded[6:10]))
	expectedLength := 4 + 2 + 4 + 8 + 4 + payloadLength + 4
	if len(encoded) != expectedLength {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: length %d does not match expected %d", len(encoded), expectedLength)
	}
	expectedCRC := binary.LittleEndian.Uint32(encoded[len(encoded)-4:])
	actualCRC := crc32.Checksum(encoded[:len(encoded)-4], catalogCRC32CTable)
	if expectedCRC != actualCRC {
		return Snapshot{}, fmt.Errorf("unmarshal snapshot: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	entryCount := int(binary.LittleEndian.Uint32(encoded[18:22]))
	entries, err := unmarshalEntries(encoded[22:22+payloadLength], entryCount)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		LastSequence: binary.LittleEndian.Uint64(encoded[10:18]),
		Entries:      entries,
	}
	return snapshot, snapshot.Validate()
}

func marshalEntries(entries []Entry) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	for _, entry := range entries {
		payload, err := marshalEntry(entry)
		if err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(payload))); err != nil {
			return nil, err
		}
		if _, err := buf.Write(payload); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func unmarshalEntries(data []byte, expectedCount int) ([]Entry, error) {
	offset := 0
	entries := make([]Entry, 0, expectedCount)
	for offset < len(data) {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("unmarshal entries: truncated entry length")
		}
		length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if offset+length > len(data) {
			return nil, fmt.Errorf("unmarshal entries: truncated entry payload")
		}
		entry, err := unmarshalEntry(data[offset : offset+length])
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
		offset += length
	}
	if len(entries) != expectedCount {
		return nil, fmt.Errorf("unmarshal entries: expected %d entries, got %d", expectedCount, len(entries))
	}
	return entries, nil
}

func marshalEntry(entry Entry) ([]byte, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	buf.Write(entry.RecordID[:])
	identity, err := marshalContentIdentity(entry.Identity)
	if err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(identity))); err != nil {
		return nil, err
	}
	buf.Write(identity)
	if err := binary.Write(buf, binary.LittleEndian, uint16(processingStateCode(entry.State))); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, entry.FirstSeenUnixNano); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, entry.LastSeenUnixNano); err != nil {
		return nil, err
	}
	if err := encodeBool(buf, entry.LastProcessedPresent); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, entry.LastProcessedUnixNano); err != nil {
		return nil, err
	}
	if len(entry.Captures) > 0xffff || len(entry.JobReferences) > 0xffff {
		return nil, fmt.Errorf("marshal entry: capture/job reference counts exceed uint16")
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(entry.Captures))); err != nil {
		return nil, err
	}
	for _, capture := range entry.Captures {
		payload, err := marshalCaptureIdentity(capture)
		if err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(payload))); err != nil {
			return nil, err
		}
		buf.Write(payload)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(entry.JobReferences))); err != nil {
		return nil, err
	}
	for _, ref := range entry.JobReferences {
		payload, err := marshalJobReference(ref)
		if err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(payload))); err != nil {
			return nil, err
		}
		buf.Write(payload)
	}
	buf.Write(entry.PreferredJobID[:])
	if err := encodeBool(buf, entry.Hidden); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unmarshalEntry(data []byte) (Entry, error) {
	offset := 0
	if len(data) < 16 {
		return Entry{}, fmt.Errorf("unmarshal entry: buffer too small")
	}
	var entry Entry
	copy(entry.RecordID[:], data[offset:offset+16])
	offset += 16
	if offset+4 > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated identity length")
	}
	identityLength := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+identityLength > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated identity payload")
	}
	identity, err := unmarshalContentIdentity(data[offset : offset+identityLength])
	if err != nil {
		return Entry{}, err
	}
	entry.Identity = identity
	offset += identityLength
	if offset+2+8+8+1+8+2 > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated fixed fields")
	}
	state, err := decodeProcessingState(binary.LittleEndian.Uint16(data[offset : offset+2]))
	if err != nil {
		return Entry{}, err
	}
	entry.State = state
	offset += 2
	entry.FirstSeenUnixNano = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8
	entry.LastSeenUnixNano = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8
	entry.LastProcessedPresent, err = decodeBool(data, &offset)
	if err != nil {
		return Entry{}, err
	}
	entry.LastProcessedUnixNano = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
	offset += 8
	captureCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	entry.Captures = make([]CaptureIdentity, 0, captureCount)
	for i := 0; i < captureCount; i++ {
		if offset+4 > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated capture length")
		}
		length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if offset+length > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated capture payload")
		}
		capture, err := unmarshalCaptureIdentity(data[offset : offset+length])
		if err != nil {
			return Entry{}, err
		}
		entry.Captures = append(entry.Captures, capture)
		offset += length
	}
	if offset+2 > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference count")
	}
	jobCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	entry.JobReferences = make([]JobReference, 0, jobCount)
	for i := 0; i < jobCount; i++ {
		if offset+4 > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference length")
		}
		length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if offset+length > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference payload")
		}
		reference, err := unmarshalJobReference(data[offset : offset+length])
		if err != nil {
			return Entry{}, err
		}
		entry.JobReferences = append(entry.JobReferences, reference)
		offset += length
	}
	if offset+16+1 > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated preferred job or hidden flag")
	}
	copy(entry.PreferredJobID[:], data[offset:offset+16])
	offset += 16
	entry.Hidden, err = decodeBool(data, &offset)
	if err != nil {
		return Entry{}, err
	}
	if offset != len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: trailing bytes %d", len(data)-offset)
	}
	return entry, entry.Validate()
}

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

func marshalJobReference(reference JobReference) ([]byte, error) {
	if err := reference.Validate(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	buf.Write(reference.JobID[:])
	if err := encodeString(buf, reference.Path); err != nil {
		return nil, err
	}
	if err := encodeBool(buf, reference.FilesPresent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unmarshalJobReference(data []byte) (JobReference, error) {
	offset := 0
	if len(data) < 16 {
		return JobReference{}, fmt.Errorf("unmarshal job reference: buffer too small")
	}
	var reference JobReference
	copy(reference.JobID[:], data[:16])
	offset += 16
	var err error
	reference.Path, err = decodeString(data, &offset)
	if err != nil {
		return JobReference{}, err
	}
	reference.FilesPresent, err = decodeBool(data, &offset)
	if err != nil {
		return JobReference{}, err
	}
	if offset != len(data) {
		return JobReference{}, fmt.Errorf("unmarshal job reference: trailing bytes %d", len(data)-offset)
	}
	return reference, reference.Validate()
}

func processingStateCode(state ProcessingState) uint16 {
	switch state {
	case ProcessingObserved:
		return 0
	case ProcessingInProgress:
		return 1
	case ProcessingStoppedResumable:
		return 2
	case ProcessingCompletedVerified:
		return 3
	case ProcessingCompletedWithGaps:
		return 4
	case ProcessingFailed:
		return 5
	case ProcessingMerged:
		return 6
	default:
		return 0xffff
	}
}

func decodeProcessingState(code uint16) (ProcessingState, error) {
	switch code {
	case 0:
		return ProcessingObserved, nil
	case 1:
		return ProcessingInProgress, nil
	case 2:
		return ProcessingStoppedResumable, nil
	case 3:
		return ProcessingCompletedVerified, nil
	case 4:
		return ProcessingCompletedWithGaps, nil
	case 5:
		return ProcessingFailed, nil
	case 6:
		return ProcessingMerged, nil
	default:
		return "", fmt.Errorf("decode processing state: unsupported code %d", code)
	}
}
