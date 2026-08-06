//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type OSRecovery struct{}

type mountedRecoveryJob struct {
	cancel context.CancelFunc

	mu       sync.Mutex
	snapshot RecoverySnapshot
}

func (j *mountedRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshot
}

func (j *mountedRecoveryJob) Cancel() {
	j.cancel()
}

func (j *mountedRecoveryJob) setProgress(copied uint64) {
	j.mu.Lock()
	j.snapshot.CopiedBytes = copied
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) addUnreadable(count uint64, lines ...string) {
	j.mu.Lock()
	j.snapshot.UnreadableSectors += count
	if len(lines) > 0 {
		j.snapshot.LastIssue = append([]string(nil), lines...)
	}
	j.mu.Unlock()
}

func (j *mountedRecoveryJob) finish(canceled bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.Done = true
	j.snapshot.Canceled = canceled
	if err != nil {
		j.snapshot.ErrText = err.Error()
	}
}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	if input.DevicePath == "" {
		return nil, fmt.Errorf("start image recovery: device path is required")
	}
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return nil, fmt.Errorf("start image recovery: output path is not configured")
	}
	if input.LogicalSectorSize == 0 {
		return nil, fmt.Errorf("start image recovery: logical sector size is required")
	}
	if input.CapacitySectors == 0 {
		return nil, fmt.Errorf("start image recovery: capacity sectors must be greater than zero")
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &mountedRecoveryJob{
		cancel: cancel,
		snapshot: RecoverySnapshot{
			StartedAt:  time.Now(),
			TotalBytes: input.CapacitySectors * uint64(input.LogicalSectorSize),
		},
	}

	go job.run(ctx, input)
	return job, nil
}

func (j *mountedRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath := rawVolumePath(input.DevicePath)
	source, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open source volume %s: %w", rawPath, err))
		return
	}
	defer source.Close()

	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			j.finish(false, fmt.Errorf("create output directory %s: %w", dir, err))
			return
		}
	}
	output, err := os.Create(input.OutputPath)
	if err != nil {
		j.finish(false, fmt.Errorf("create output image %s: %w", input.OutputPath, err))
		return
	}
	defer output.Close()

	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if err := output.Truncate(int64(totalBytes)); err != nil {
		j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
		return
	}

	chunkSize := int(input.LogicalSectorSize) * 64
	if chunkSize < int(input.LogicalSectorSize) {
		chunkSize = int(input.LogicalSectorSize)
	}
	buffer := make([]byte, chunkSize)
	var copied uint64
	logicalSectorSize := uint64(input.LogicalSectorSize)

	for lba := uint64(0); lba < input.CapacitySectors; {
		select {
		case <-ctx.Done():
			j.setProgress(copied)
			j.finish(true, nil)
			return
		default:
		}

		sectorsToRead := uint64(64)
		if remaining := input.CapacitySectors - lba; remaining < sectorsToRead {
			sectorsToRead = remaining
		}
		readSize := int(sectorsToRead * logicalSectorSize)
		offset := int64(lba * logicalSectorSize)

		n, err := source.ReadAt(buffer[:readSize], offset)
		if err == nil || (err == io.EOF && n == readSize) {
			if err := writeFullAt(output, buffer[:readSize], offset); err != nil {
				j.setProgress(copied)
				j.finish(false, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, offset, err))
				return
			}
			copied += uint64(readSize)
			j.setProgress(copied)
			lba += sectorsToRead
			continue
		}

		singleSector := int(input.LogicalSectorSize)
		sectorBuffer := make([]byte, singleSector)
		clusterStart := lba
		clusterEnd := lba + sectorsToRead
		for lba < clusterEnd {
			select {
			case <-ctx.Done():
				j.setProgress(copied)
				j.finish(true, nil)
				return
			default:
			}

			sectorOffset := int64(lba * logicalSectorSize)
			n, err := source.ReadAt(sectorBuffer, sectorOffset)
			if err == nil || (err == io.EOF && n == singleSector) {
				if err := writeFullAt(output, sectorBuffer[:singleSector], sectorOffset); err != nil {
					j.setProgress(copied)
					j.finish(false, fmt.Errorf("write output image %s at byte %d: %w", input.OutputPath, sectorOffset, err))
					return
				}
				copied += logicalSectorSize
				j.setProgress(copied)
				lba++
				continue
			}

			j.addUnreadable(1,
				fmt.Sprintf("Unreadable sector at LBA %d.", lba),
				fmt.Sprintf("Cluster fallback was required for LBA %d to %d.", clusterStart, clusterEnd-1),
			)
			lba++
		}
	}

	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func writeFullAt(file *os.File, data []byte, offset int64) error {
	written := 0
	for written < len(data) {
		n, err := file.WriteAt(data[written:], offset+int64(written))
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("short write")
		}
	}
	return nil
}
