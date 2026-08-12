package recoverymap

import (
	"errors"
	"fmt"
	"os"

	"discrescue/internal/mapfile"
)

func (s *Store) Close(clean bool) error {
	if s == nil || s.file == nil || s.closed {
		return nil
	}
	if err := s.Flush(); err != nil {
		closeErr := s.file.Close()
		s.closed = true
		return errors.Join(err, closeErr)
	}
	s.header.CleanShutdown = clean
	headerBytes, err := mapfile.MarshalHeader(s.header)
	if err != nil {
		return s.failClose(fmt.Errorf("finalize recovery map %s: %w", s.path, err))
	}
	if err := s.writeAt(0, headerBytes); err != nil {
		return s.failClose(fmt.Errorf("finalize recovery map %s: %w", s.path, err))
	}
	if err := s.file.Sync(); err != nil {
		closeErr := s.file.Close()
		s.closed = true
		return errors.Join(fmt.Errorf("sync finalized recovery map %s: %w", s.path, err), closeErr)
	}
	s.closed = true
	return s.file.Close()
}
func (s *Store) abort(cause error) error {
	closeErr := s.file.Close()
	return errors.Join(cause, closeErr)
}
func (s *Store) abortCreate(cause error) error {
	closeErr := s.file.Close()
	removeErr := os.Remove(s.path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(cause, closeErr, removeErr)
}
func (s *Store) failClose(cause error) error {
	closeErr := s.file.Close()
	s.closed = true
	return errors.Join(cause, closeErr)
}
