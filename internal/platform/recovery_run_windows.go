//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"discrescue/internal/recovery"
)

func (j *mountedRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath := rawVolumePath(input.DevicePath)
	sourceFile, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open source volume %s: %w", rawPath, err))
		return
	}
	source, err := recovery.NewReopenableReaderAt(sourceFile, sourceFile.Close, func() (io.ReaderAt, error) {
		return os.Open(rawPath)
	})
	if err != nil {
		_ = sourceFile.Close()
		j.finish(false, fmt.Errorf("prepare source volume %s: %w", rawPath, err))
		return
	}
	defer source.Close()
	j.mu.Lock()
	j.source = source
	j.mu.Unlock()

	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			j.finish(false, fmt.Errorf("create output directory %s: %w", dir, err))
			return
		}
	}
	flags := os.O_RDWR
	if j.startup != nil {
		flags |= os.O_CREATE | os.O_EXCL
	} else {
		flags |= os.O_CREATE
	}
	output, err := os.OpenFile(input.OutputPath, flags, 0o644)
	if err != nil {
		j.finish(false, fmt.Errorf("create output image %s: %w", input.OutputPath, err))
		return
	}
	if j.startup != nil {
		j.startup.TrackCreated(input.OutputPath)
	}
	defer output.Close()

	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if err := output.Truncate(int64(totalBytes)); err != nil {
		j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
		return
	}
	if j.startup != nil {
		j.startup.Commit()
	}

	persistence := newRecoveryPersistence(output, j.state)
	lifecycleSource := &recovery.LifecycleReaderAt{Source: source, Lifecycle: j.lifecycle}
	policy, policyErr := recovery.PolicyForMethod(input.Method)
	if policyErr != nil {
		j.finish(false, policyErr)
		return
	}
	if input.RetryUnresolved {
		policy = retryPolicyWithCurrentAttempts(policy, j.state.Extents())
	}
	err = runPassBasedRecoveryWithPolicy(
		ctx,
		lifecycleSource,
		output,
		input.LogicalSectorSize,
		input.CapacitySectors,
		persistence,
		policy,
		func(progress recoveryPassProgress) {
			j.setPassProgress(progress, input.LogicalSectorSize)
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			j.finish(true, nil)
			return
		}
		j.finish(false, fmt.Errorf("recover image %s: %w", input.OutputPath, err))
		return
	}

	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}
