package recoverymap

import (
	"io"
	"os"
)

func writeFull(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

// Close finalizes the session header and closes the map. A clean state is
// recorded only after the header write and map sync both succeed.
func (s *Store) writeAt(offset int64, data []byte) error {
	written, err := s.file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
func readAtExact(file *os.File, offset int64, data []byte) error {
	read, err := file.ReadAt(data, offset)
	if err != nil {
		return err
	}
	if read != len(data) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
