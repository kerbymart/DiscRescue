package recoverymap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartupTransactionRollsBackCreatedArtifacts(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("created"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var tx StartupTransaction
	tx.TrackCreated(first)
	tx.TrackCreated(second)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("artifact %s remains: %v", path, err)
		}
	}
}

func TestStartupTransactionCommitPreservesCreatedArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, []byte("created"), 0o600); err != nil {
		t.Fatal(err)
	}
	var tx StartupTransaction
	tx.TrackCreated(path)
	tx.Commit()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
