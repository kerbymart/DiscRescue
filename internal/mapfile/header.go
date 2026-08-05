package mapfile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const headerLength = 135

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

type Header struct {
	LogicalSectorSize        uint32
	ExpectedSectorCount      uint64
	OutputFormat             uint16
	IdentityAlgorithmVersion uint16
	LayoutSHA256             [32]byte
	QuickContentIDPresent    bool
	QuickContentID           [16]byte
	CaptureID                [16]byte
	CatalogRecordIDPresent   bool
	CatalogRecordID          [16]byte
	JobID                    [16]byte
	CreationUnixNano         int64
	CleanShutdown            bool
}

func (h Header) Validate() error {
	if h.LogicalSectorSize == 0 {
		return fmt.Errorf("validate header: logical sector size must be greater than zero")
	}
	if h.ExpectedSectorCount == 0 {
		return fmt.Errorf("validate header: expected sector count must be greater than zero")
	}
	return nil
}

func MarshalHeader(header Header) ([]byte, error) {
	if err := header.Validate(); err != nil {
		return nil, err
	}

	encoded := make([]byte, headerLength)
	copy(encoded[0:4], []byte(HeaderMagic))
	binary.LittleEndian.PutUint16(encoded[4:6], FormatVersion)
	binary.LittleEndian.PutUint16(encoded[6:8], headerLength)
	binary.LittleEndian.PutUint32(encoded[8:12], header.LogicalSectorSize)
	binary.LittleEndian.PutUint64(encoded[12:20], header.ExpectedSectorCount)
	binary.LittleEndian.PutUint16(encoded[20:22], header.OutputFormat)
	binary.LittleEndian.PutUint16(encoded[22:24], header.IdentityAlgorithmVersion)
	copy(encoded[24:56], header.LayoutSHA256[:])
	if header.QuickContentIDPresent {
		encoded[56] = 1
	}
	copy(encoded[57:73], header.QuickContentID[:])
	copy(encoded[73:89], header.CaptureID[:])
	if header.CatalogRecordIDPresent {
		encoded[89] = 1
	}
	copy(encoded[90:106], header.CatalogRecordID[:])
	copy(encoded[106:122], header.JobID[:])
	binary.LittleEndian.PutUint64(encoded[122:130], uint64(header.CreationUnixNano))
	if header.CleanShutdown {
		encoded[130] = 1
	}
	crc := crc32.Checksum(encoded[:headerLength-4], crc32cTable)
	binary.LittleEndian.PutUint32(encoded[headerLength-4:], crc)
	return encoded, nil
}

func UnmarshalHeader(encoded []byte) (Header, error) {
	if len(encoded) < headerLength {
		return Header{}, fmt.Errorf("unmarshal header: expected %d bytes, got %d", headerLength, len(encoded))
	}
	if string(encoded[0:4]) != HeaderMagic {
		return Header{}, fmt.Errorf("unmarshal header: unexpected magic %q", string(encoded[0:4]))
	}
	if version := binary.LittleEndian.Uint16(encoded[4:6]); version != FormatVersion {
		return Header{}, fmt.Errorf("unmarshal header: unsupported version %d", version)
	}
	if length := binary.LittleEndian.Uint16(encoded[6:8]); length != headerLength {
		return Header{}, fmt.Errorf("unmarshal header: unsupported header length %d", length)
	}
	expectedCRC := binary.LittleEndian.Uint32(encoded[headerLength-4 : headerLength])
	actualCRC := crc32.Checksum(encoded[:headerLength-4], crc32cTable)
	if expectedCRC != actualCRC {
		return Header{}, fmt.Errorf("unmarshal header: crc32c mismatch expected %08x actual %08x", expectedCRC, actualCRC)
	}

	var header Header
	header.LogicalSectorSize = binary.LittleEndian.Uint32(encoded[8:12])
	header.ExpectedSectorCount = binary.LittleEndian.Uint64(encoded[12:20])
	header.OutputFormat = binary.LittleEndian.Uint16(encoded[20:22])
	header.IdentityAlgorithmVersion = binary.LittleEndian.Uint16(encoded[22:24])
	copy(header.LayoutSHA256[:], encoded[24:56])
	header.QuickContentIDPresent = encoded[56] == 1
	copy(header.QuickContentID[:], encoded[57:73])
	copy(header.CaptureID[:], encoded[73:89])
	header.CatalogRecordIDPresent = encoded[89] == 1
	copy(header.CatalogRecordID[:], encoded[90:106])
	copy(header.JobID[:], encoded[106:122])
	header.CreationUnixNano = int64(binary.LittleEndian.Uint64(encoded[122:130]))
	header.CleanShutdown = encoded[130] == 1

	return header, header.Validate()
}
