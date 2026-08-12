package platform

func classifyRecoveryReadError(err error) error {
	if platformFatalSourceReadError(err) {
		return err
	}
	return nil
}
