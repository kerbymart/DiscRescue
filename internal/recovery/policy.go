package recovery

import (
	"fmt"
	"time"
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
			FinalizeUnresolved: false}
	case RecoveryMethodBalanced:
		policy = RecoveryPolicy{Method: method,
			Fast:               FastPassPolicy{Enabled: true, BlockSectors: 64},
			Trim:               TrimPassPolicy{Enabled: true, AttemptsLimit: 2},
			Adaptive:           []AdaptivePassPolicy{{Enabled: true, BlockSectors: 16, AttemptsLimit: 3}, {Enabled: true, BlockSectors: 4, AttemptsLimit: 4}},
			Targeted:           TargetedPassPolicy{Enabled: true, BlockSectors: 1, AttemptsLimit: 6},
			FinalizeUnresolved: true}
	case RecoveryMethodGentle:
		policy = RecoveryPolicy{Method: method,
			Fast:               FastPassPolicy{Enabled: true, BlockSectors: 32},
			Trim:               TrimPassPolicy{Enabled: true, AttemptsLimit: 1},
			Adaptive:           []AdaptivePassPolicy{{Enabled: true, BlockSectors: 8, AttemptsLimit: 2}},
			Targeted:           TargetedPassPolicy{Enabled: true, BlockSectors: 1, AttemptsLimit: 3},
			FinalizeUnresolved: false}
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
	return nil
}
