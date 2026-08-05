package device

type MediaInfo struct {
	LogicalSectorSize uint32
	CapacitySectors   uint64
	Profile           string
}

type CommandKind string

const (
	CommandProbe CommandKind = "probe"
	CommandRead  CommandKind = "read"
)

type CommandRequest struct {
	ID       uint64
	Command  CommandKind
	StartLBA uint64
	Sectors  uint32
}

type CommandResult struct {
	ID     uint64
	Status string
	Data   []byte
}
