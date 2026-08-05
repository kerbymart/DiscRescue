package device

type MediaInfo struct {
	LogicalSectorSize uint32
	CapacitySectors   uint64
	Profile           string
}

type CommandKind string

const (
	CommandInquiry             CommandKind = "inquiry"
	CommandTestReady           CommandKind = "test_ready"
	CommandGetConfiguration    CommandKind = "get_configuration"
	CommandReadCapacity        CommandKind = "read_capacity"
	CommandReadTOC             CommandKind = "read_toc"
	CommandReadDiscInformation CommandKind = "read_disc_information"
	CommandReadDVDStructure    CommandKind = "read_dvd_structure"
	CommandReadBlocks          CommandKind = "read_blocks"
	CommandReadCD              CommandKind = "read_cd"
	CommandSetSpeed            CommandKind = "set_speed"
	CommandEject               CommandKind = "eject"
)

func (k CommandKind) IsReadOnly() bool {
	switch k {
	case CommandInquiry,
		CommandTestReady,
		CommandGetConfiguration,
		CommandReadCapacity,
		CommandReadTOC,
		CommandReadDiscInformation,
		CommandReadDVDStructure,
		CommandReadBlocks,
		CommandReadCD,
		CommandSetSpeed,
		CommandEject:
		return true
	default:
		return false
	}
}

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
