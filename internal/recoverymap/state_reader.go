package recoverymap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"discrescue/internal/mapfile"
)

func Inspect(path string, geometry Geometry) (mapfile.Header, []mapfile.Extent, error) {
	header, replayed, err := readState(path, geometry)
	if err != nil {
		return mapfile.Header{}, nil, fmt.Errorf("inspect recovery map %s: %w", path, err)
	}
	return header, append([]mapfile.Extent(nil), replayed.Extents...), nil
}

// Path returns the map's filesystem path.
func readState(path string, geometry Geometry) (mapfile.Header, mapfile.Checkpoint, error) {
	if geometry.LogicalSectorSize == 0 || geometry.ExpectedSectorCount == 0 {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("media geometry is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("open: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("stat: %w", err)
	}
	if info.Size() < 8 {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map is too short")
	}
	prefix := make([]byte, 8)
	if err := readAtExact(file, 0, prefix); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read prefix: %w", err)
	}
	headerLength := int(binary.LittleEndian.Uint16(prefix[6:8]))
	if headerLength <= 0 {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map header length is invalid")
	}
	headerBytes := make([]byte, headerLength)
	if err := readAtExact(file, 0, headerBytes); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read header: %w", err)
	}
	header, err := mapfile.UnmarshalHeader(headerBytes)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	checkpointPrefix := make([]byte, 10)
	if err := readAtExact(file, int64(headerLength), checkpointPrefix); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read checkpoint header: %w", err)
	}
	payloadLength := uint64(binary.LittleEndian.Uint32(checkpointPrefix[6:10]))
	checkpointLength := uint64(14) + payloadLength
	if checkpointLength > uint64(info.Size())-uint64(headerLength) {
		return mapfile.Header{}, mapfile.Checkpoint{}, errors.New("recovery map checkpoint is truncated")
	}
	if payloadLength > mapfile.DefaultDecodeLimits.MaxCheckpointPayloadBytes {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("recovery map checkpoint payload %d exceeds limit", payloadLength)
	}
	checkpointBytes := make([]byte, int(checkpointLength))
	if err := readAtExact(file, int64(headerLength), checkpointBytes); err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("read checkpoint: %w", err)
	}
	checkpoint, err := mapfile.UnmarshalCheckpoint(checkpointBytes)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	journalOffset := uint64(headerLength) + checkpointLength
	if header.LogicalSectorSize != geometry.LogicalSectorSize || header.ExpectedSectorCount != geometry.ExpectedSectorCount {
		return mapfile.Header{}, mapfile.Checkpoint{}, fmt.Errorf("map geometry does not match media")
	}
	journalReader := io.NewSectionReader(file, int64(journalOffset), info.Size()-int64(journalOffset))
	replayed, err := mapfile.ReplayJournalReaderWithinCapacity(checkpoint, journalReader, geometry.ExpectedSectorCount)
	if err != nil {
		return mapfile.Header{}, mapfile.Checkpoint{}, err
	}
	return header, replayed, nil
}
