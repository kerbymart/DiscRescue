package recovery

import (
	"fmt"
	"time"
)

const (
	defaultHealthySoftDeadline = 5 * time.Second
	defaultHealthyHardDeadline = 30 * time.Second
	defaultDamagedSoftDeadline = 2 * time.Second
	defaultDamagedHardDeadline = 10 * time.Second
)

type FastPassPolicy struct {
	Enabled      bool
	BlockSectors uint32
}

type TrimPassPolicy struct {
	Enabled       bool
	AttemptsLimit uint16
}

type AdaptivePassPolicy struct {
	Enabled       bool
	BlockSectors  uint32
	AttemptsLimit uint16
}

type TargetedPassPolicy struct {
	Enabled       bool
	BlockSectors  uint32
	AttemptsLimit uint16
}

type ReadDeadlinePolicy struct {
	HealthySoft time.Duration
	HealthyHard time.Duration
	DamagedSoft time.Duration
	DamagedHard time.Duration
}

// RecoveryPolicy is immutable after validation and controls scheduler passes,
// not physical optical-drive speed.
type RecoveryPolicy struct {
	Method             RecoveryMethod
	Fast               FastPassPolicy
	Trim               TrimPassPolicy
	Adaptive           []AdaptivePassPolicy
	Targeted           TargetedPassPolicy
	FinalizeUnresolved bool
	ReadDeadlines      ReadDeadlinePolicy
}

func PolicyForMethod(method RecoveryMethod) (RecoveryPolicy, error) {
	if method == "" {
		method = RecoveryMethodBalanced
	}
	var policy RecoveryPolicy
	switch method {
	case RecoveryMethodFast:
		policy = RecoveryPolicy{Method: method,
			Fast:               FastPassPolicy{Enabled: true, BlockSectors: 64},
			Trim:               TrimPassPolicy{Enabled: true, AttemptsLimit: 1},
			Adaptive:           []AdaptivePassPolicy{{Enabled: true, BlockSectors: 16, AttemptsLimit: 2}},
			FinalizeUnresolved: false,
			ReadDeadlines:      defaultReadDeadlinePolicy()}
	case RecoveryMethodBalanced:
		policy = RecoveryPolicy{Method: method,
			Fast:               FastPassPolicy{Enabled: true, BlockSectors: 64},
			Trim:               TrimPassPolicy{Enabled: true, AttemptsLimit: 2},
			Adaptive:           []AdaptivePassPolicy{{Enabled: true, BlockSectors: 16, AttemptsLimit: 3}, {Enabled: true, BlockSectors: 4, AttemptsLimit: 4}},
			Targeted:           TargetedPassPolicy{Enabled: true, BlockSectors: 1, AttemptsLimit: 6},
			FinalizeUnresolved: true,
			ReadDeadlines:      defaultReadDeadlinePolicy()}
	case RecoveryMethodGentle:
		policy = RecoveryPolicy{Method: method,
			Fast:               FastPassPolicy{Enabled: true, BlockSectors: 32},
			Trim:               TrimPassPolicy{Enabled: true, AttemptsLimit: 1},
			Adaptive:           []AdaptivePassPolicy{{Enabled: true, BlockSectors: 8, AttemptsLimit: 2}},
			Targeted:           TargetedPassPolicy{Enabled: true, BlockSectors: 1, AttemptsLimit: 3},
			FinalizeUnresolved: false,
			ReadDeadlines:      defaultReadDeadlinePolicy()}
	default:
		return RecoveryPolicy{}, fmt.Errorf("policy for method: unknown recovery method %q", method)
	}
	if err := policy.Validate(); err != nil {
		return RecoveryPolicy{}, err
	}
	return policy, nil
}

func (p RecoveryPolicy) Validate() error {
	if p.Method != RecoveryMethodFast && p.Method != RecoveryMethodBalanced && p.Method != RecoveryMethodGentle {
		return fmt.Errorf("validate recovery policy: unknown method %q", p.Method)
	}
	if p.Fast.Enabled && p.Fast.BlockSectors == 0 {
		return fmt.Errorf("validate recovery policy: fast block size must be greater than zero")
	}
	if p.Trim.Enabled && p.Trim.AttemptsLimit == 0 {
		return fmt.Errorf("validate recovery policy: trim attempts limit must be greater than zero")
	}
	for i, pass := range p.Adaptive {
		if !pass.Enabled {
			continue
		}
		if pass.BlockSectors == 0 || pass.AttemptsLimit == 0 {
			return fmt.Errorf("validate recovery policy: adaptive pass %d has invalid block size or attempts limit", i)
		}
	}
	if p.Targeted.Enabled && (p.Targeted.BlockSectors == 0 || p.Targeted.AttemptsLimit == 0) {
		return fmt.Errorf("validate recovery policy: targeted pass has invalid block size or attempts limit")
	}
	if p.Targeted.Enabled && len(p.Adaptive) > 0 {
		last := p.Adaptive[len(p.Adaptive)-1]
		if last.Enabled && p.Targeted.BlockSectors > last.BlockSectors {
			return fmt.Errorf("validate recovery policy: targeted block size exceeds final adaptive block size")
		}
	}
	if err := p.ReadDeadlines.Validate(); err != nil {
		return fmt.Errorf("validate recovery policy: %w", err)
	}
	return nil
}

func defaultReadDeadlinePolicy() ReadDeadlinePolicy {
	return ReadDeadlinePolicy{
		HealthySoft: defaultHealthySoftDeadline,
		HealthyHard: defaultHealthyHardDeadline,
		DamagedSoft: defaultDamagedSoftDeadline,
		DamagedHard: defaultDamagedHardDeadline,
	}
}

func (p ReadDeadlinePolicy) Validate() error {
	if p.HealthySoft <= 0 || p.HealthyHard <= 0 || p.DamagedSoft <= 0 || p.DamagedHard <= 0 {
		return fmt.Errorf("read deadlines must all be positive")
	}
	if p.HealthySoft > p.HealthyHard {
		return fmt.Errorf("healthy soft deadline exceeds hard deadline")
	}
	if p.DamagedSoft > p.DamagedHard {
		return fmt.Errorf("damaged soft deadline exceeds hard deadline")
	}
	return nil
}
