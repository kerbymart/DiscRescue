package platform

import "discrescue/internal/catalog"

func ensureIdentityObservation(media OpticalMedia) OpticalMedia {
	if media.IdentityObservation == nil {
		media.IdentityObservation = &catalog.IdentityObservation{Status: catalog.IdentityUnavailable, Detail: "Content fingerprint collection is unavailable for this media inspection."}
	}
	return media
}
