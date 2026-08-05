package merge

import (
	"testing"

	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

func BenchmarkBuildPlan(b *testing.B) {
	captures := []Capture{
		{
			CaptureID: "capture-a",
			Identity:  benchmarkIdentity(),
			Extents: []mapfile.Extent{
				mergeExtent(0, 2048, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "same"),
			},
		},
		{
			CaptureID: "capture-b",
			Identity:  benchmarkIdentity(),
			Extents: []mapfile.Extent{
				mergeExtent(0, 2048, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "same"),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildPlan(captures); err != nil {
			b.Fatalf("build plan: %v", err)
		}
	}
}

func benchmarkIdentity() catalog.ContentIdentity {
	return catalog.ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      2048,
		Sessions:         1,
		Tracks: []catalog.TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 2047, Mode: 1, LeadOutLBA: 2048},
		},
		LayoutSHA256: "layout-benchmark",
	}
}
