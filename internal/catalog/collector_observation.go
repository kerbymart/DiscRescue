package catalog

import "context"

func (c FingerprintCollector) CollectObservation(ctx context.Context, base ContentIdentity, reader SectorReader) (IdentityObservation, error) {
	identity, stats, err := c.Collect(ctx, base, reader)
	if err != nil {
		return IdentityObservation{Status: IdentityUnavailable, Detail: err.Error()}, err
	}
	status := IdentityInsufficientEvidence
	detail := "Not enough readable fingerprint samples are available for a reliable match."
	if identity.QuickID != "" {
		status = IdentityStrongEvidence
		detail = "All bounded fingerprint samples were collected."
	} else if stats.AvailableSamples > 0 {
		status = IdentityPartialEvidence
		detail = "Some fingerprint samples were unavailable; automatic matching is restricted."
	}
	return IdentityObservation{
		Identity:           identity,
		Status:             status,
		AttemptedSamples:   stats.AttemptedSamples,
		AvailableSamples:   stats.AvailableSamples,
		UnavailableSamples: stats.UnavailableSamples,
		BytesRead:          stats.BytesRead,
		CollectionDuration: stats.Duration,
		Detail:             detail,
	}, nil
}
