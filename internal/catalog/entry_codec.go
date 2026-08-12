package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

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
	if expectedCount < 0 || expectedCount > maxCatalogEntries {
		return nil, fmt.Errorf("unmarshal entries: entry count %d exceeds limit %d", expectedCount, maxCatalogEntries)
	}
	if uint64(expectedCount)*4 > uint64(len(data)) {
		return nil, fmt.Errorf("unmarshal entries: entry count %d cannot fit payload", expectedCount)
	}
	offset := 0
	entries := make([]Entry, 0, expectedCount)
	for offset < len(data) {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("unmarshal entries: truncated entry length")
		}
		length := uint64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if length > uint64(len(data)-offset) {
			return nil, fmt.Errorf("unmarshal entries: truncated entry payload")
		}
		entry, err := unmarshalEntry(data[offset : offset+int(length)])
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
		offset += int(length)
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
	identityLength := uint64(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if identityLength > maxCatalogPayloadBytes || identityLength > uint64(len(data)-offset) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated identity payload")
	}
	identity, err := unmarshalContentIdentity(data[offset : offset+int(identityLength)])
	if err != nil {
		return Entry{}, err
	}
	entry.Identity = identity
	offset += int(identityLength)
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
	if captureCount > maxCatalogCapturesPerEntry || uint64(captureCount)*4 > uint64(len(data)-offset) {
		return Entry{}, fmt.Errorf("unmarshal entry: capture count %d exceeds bounds", captureCount)
	}
	entry.Captures = make([]CaptureIdentity, 0, captureCount)
	for i := 0; i < captureCount; i++ {
		if offset+4 > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated capture length")
		}
		length := uint64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if length > uint64(len(data)-offset) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated capture payload")
		}
		capture, err := unmarshalCaptureIdentity(data[offset : offset+int(length)])
		if err != nil {
			return Entry{}, err
		}
		entry.Captures = append(entry.Captures, capture)
		offset += int(length)
	}
	if offset+2 > len(data) {
		return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference count")
	}
	jobCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if jobCount > maxCatalogJobsPerEntry || uint64(jobCount)*4 > uint64(len(data)-offset) {
		return Entry{}, fmt.Errorf("unmarshal entry: job reference count %d exceeds bounds", jobCount)
	}
	entry.JobReferences = make([]JobReference, 0, jobCount)
	for i := 0; i < jobCount; i++ {
		if offset+4 > len(data) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference length")
		}
		length := uint64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if length > uint64(len(data)-offset) {
			return Entry{}, fmt.Errorf("unmarshal entry: truncated job reference payload")
		}
		reference, err := unmarshalJobReference(data[offset : offset+int(length)])
		if err != nil {
			return Entry{}, err
		}
		entry.JobReferences = append(entry.JobReferences, reference)
		offset += int(length)
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
