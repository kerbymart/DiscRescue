package catalog

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func encodeString(buf *bytes.Buffer, value string) error {
	if len(value) > 0xffff {
		return fmt.Errorf("encode string: length %d exceeds uint16", len(value))
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(value))); err != nil {
		return err
	}
	_, err := buf.WriteString(value)
	return err
}
func decodeString(data []byte, offset *int) (string, error) {
	if *offset+2 > len(data) {
		return "", fmt.Errorf("decode string: truncated length")
	}
	length := int(binary.LittleEndian.Uint16(data[*offset : *offset+2]))
	*offset += 2
	if *offset+length > len(data) {
		return "", fmt.Errorf("decode string: truncated value")
	}
	value := string(data[*offset : *offset+length])
	*offset += length
	return value, nil
}
func encodeBool(buf *bytes.Buffer, value bool) error {
	var encoded byte
	if value {
		encoded = 1
	}
	return buf.WriteByte(encoded)
}
func decodeBool(data []byte, offset *int) (bool, error) {
	if *offset+1 > len(data) {
		return false, fmt.Errorf("decode bool: truncated value")
	}
	value := data[*offset]
	*offset++
	if value > 1 {
		return false, fmt.Errorf("decode bool: invalid value %d", value)
	}
	return value == 1, nil
}
