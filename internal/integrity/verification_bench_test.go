package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"discrescue/internal/mapfile"
)

func BenchmarkVerifyExternal(b *testing.B) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 251)
	}
	sum := sha256.Sum256(data)

	input := ExternalVerificationInput{
		LogicalSectorSize: 2048,
		Extents: []ExternalExtentInput{{
			Extent: mapfile.Extent{
				StartLBA:   0,
				Sectors:    1,
				State:      mapfile.SectorStateReadUnverified,
				Confidence: mapfile.ConfidenceSingleRead,
			},
			Data: data,
			Digests: []Digest{{
				Provider: ProviderSHA256,
				Value:    hex.EncodeToString(sum[:]),
			}},
		}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := VerifyExternal(input); err != nil {
			b.Fatalf("verify external: %v", err)
		}
	}
}

func BenchmarkVerifyImage(b *testing.B) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte((i * 7) % 251)
	}

	input := ImageVerificationInput{
		LogicalSectorSize: 2048,
		Extents: []ImageExtentInput{{
			Extent: mapfile.Extent{
				StartLBA:   0,
				Sectors:    1,
				State:      mapfile.SectorStateReadUnverified,
				Confidence: mapfile.ConfidenceSingleRead,
				DataHash:   hash16(data),
			},
			Data: data,
		}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := VerifyImage(input); err != nil {
			b.Fatalf("verify image: %v", err)
		}
	}
}
