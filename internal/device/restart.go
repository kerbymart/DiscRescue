package device

type RestartPolicy string

const (
	RestartNever     RestartPolicy = "never"
	RestartRetryable RestartPolicy = "retryable"
	RestartAlways    RestartPolicy = "always"
)

func (s Supervisor) RestartDecision(retryable bool) []DispatchEffect {
	switch s.RestartPolicy {
	case RestartAlways:
		return []DispatchEffect{{Kind: EffectRestartWorker, Drive: s.OwnedDrive}}
	case RestartRetryable:
		if retryable {
			return []DispatchEffect{{Kind: EffectRestartWorker, Drive: s.OwnedDrive}}
		}
	}
	return nil
}
