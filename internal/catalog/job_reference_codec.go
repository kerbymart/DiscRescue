package catalog

import (
	"bytes"
	"fmt"
)

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
