//go:build linux

package platform

import (
	"context"
	"discrescue/internal/recovery"
	"errors"
	"fmt"
)

func (j *linuxRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	source, err := openLinuxSource(input.DevicePath)
	if err != nil {
		j.finish(false, err)
		return
	}
	defer source.Close()
	j.mu.Lock()
	j.source = source
	j.mu.Unlock()
	output, err := openLinuxOutput(input, j.startup)
	if err != nil {
		j.finish(false, err)
		return
	}
	defer output.Close()
	persistence := newRecoveryPersistence(output, j.state)
	lifecycleSource := &recovery.LifecycleReaderAt{Source: source, Lifecycle: j.lifecycle}
	policy, err := recovery.PolicyForMethod(input.Method)
	if err != nil {
		j.finish(false, err)
		return
	}
	if input.RetryUnresolved {
		policy = retryPolicyWithCurrentAttempts(policy, j.state.Extents())
	}
	err = runPassBasedRecoveryWithPolicy(ctx, lifecycleSource, output, input.LogicalSectorSize, input.CapacitySectors, persistence, policy, func(progress recoveryPassProgress) { j.report(progress, input.LogicalSectorSize) })
	if errors.Is(err, context.Canceled) {
		j.finish(true, nil)
		return
	}
	if err != nil {
		j.finish(false, fmt.Errorf("recover Linux image %s: %w", input.OutputPath, err))
		return
	}
	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}
