package mapfile

const checkpointMagic = "DSCP"

const (
	checkpointFixedPayloadBytes      = uint64(8 + 4)
	defaultMaxCheckpointPayloadBytes = uint64(64 << 20)
	defaultMaxCheckpointExtents      = uint64(1 << 20)
)

// DecodeLimits bounds allocations made while decoding a checkpoint.
type DecodeLimits struct {
	MaxCheckpointPayloadBytes uint64
	MaxCheckpointExtents      uint64
}

// DefaultDecodeLimits are deliberately finite parser limits for v1 maps.
var DefaultDecodeLimits = DecodeLimits{
	MaxCheckpointPayloadBytes: defaultMaxCheckpointPayloadBytes,
	MaxCheckpointExtents:      defaultMaxCheckpointExtents,
}

type Checkpoint struct {
	LastSequence uint64
	Extents      []Extent
}
