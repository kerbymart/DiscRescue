package recovery

// RecoveryMethod is the stable user-selected recovery strategy.
type RecoveryMethod string

const (
	RecoveryMethodFast     RecoveryMethod = "fast"
	RecoveryMethodBalanced RecoveryMethod = "balanced"
	RecoveryMethodGentle   RecoveryMethod = "gentle"
)
