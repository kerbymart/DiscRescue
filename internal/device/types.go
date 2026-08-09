package device

import "fmt"

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

type CommandSpec struct {
	Kind          CommandKind
	DisplayName   string
	ReadOnly      bool
	AllowsMediaIO bool
	Opcode        byte
}

var allowedCommandSpecs = map[CommandKind]CommandSpec{
	CommandInquiry: {
		Kind:          CommandInquiry,
		DisplayName:   "INQUIRY",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x12,
	},
	CommandTestReady: {
		Kind:          CommandTestReady,
		DisplayName:   "TEST UNIT READY",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x00,
	},
	CommandGetConfiguration: {
		Kind:          CommandGetConfiguration,
		DisplayName:   "GET CONFIGURATION",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x46,
	},
	CommandReadCapacity: {
		Kind:          CommandReadCapacity,
		DisplayName:   "READ CAPACITY",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x25,
	},
	CommandReadTOC: {
		Kind:          CommandReadTOC,
		DisplayName:   "READ TOC/PMA/ATIP",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x43,
	},
	CommandReadDiscInformation: {
		Kind:          CommandReadDiscInformation,
		DisplayName:   "READ DISC INFORMATION",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x51,
	},
	CommandReadDVDStructure: {
		Kind:          CommandReadDVDStructure,
		DisplayName:   "READ DVD STRUCTURE",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0xad,
	},
	CommandReadBlocks: {
		Kind:          CommandReadBlocks,
		DisplayName:   "READ(10/12/16)",
		ReadOnly:      true,
		AllowsMediaIO: true,
		Opcode:        0x28,
	},
	CommandReadCD: {
		Kind:          CommandReadCD,
		DisplayName:   "READ CD",
		ReadOnly:      true,
		AllowsMediaIO: true,
		Opcode:        0xbe,
	},
	CommandSetSpeed: {
		Kind:          CommandSetSpeed,
		DisplayName:   "SET CD SPEED",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0xbb,
	},
	CommandEject: {
		Kind:          CommandEject,
		DisplayName:   "START STOP UNIT",
		ReadOnly:      true,
		AllowsMediaIO: false,
		Opcode:        0x1b,
	},
}

func AllowedCommandSpec(kind CommandKind) (CommandSpec, bool) {
	spec, ok := allowedCommandSpecs[kind]
	return spec, ok
}

func AllowedCommandKinds() []CommandKind {
	return []CommandKind{
		CommandInquiry,
		CommandTestReady,
		CommandGetConfiguration,
		CommandReadCapacity,
		CommandReadTOC,
		CommandReadDiscInformation,
		CommandReadDVDStructure,
		CommandReadBlocks,
		CommandReadCD,
		CommandSetSpeed,
		CommandEject,
	}
}

func (k CommandKind) IsReadOnly() bool {
	spec, ok := AllowedCommandSpec(k)
	return ok && spec.ReadOnly
}

func ValidateCommandKind(kind CommandKind) error {
	if _, ok := AllowedCommandSpec(kind); !ok {
		return fmt.Errorf("validate command kind: command %q is not in the allowlist", kind)
	}
	return nil
}

type CommandRequest struct {
	ID        uint64
	Command   CommandKind
	StartLBA  uint64
	Sectors   uint32
	SpeedKbps uint32
}

func (r CommandRequest) Validate() error {
	if r.Command == "" {
		return fmt.Errorf("validate command request: command is required")
	}
	if err := ValidateCommandKind(r.Command); err != nil {
		return err
	}
	if spec, _ := AllowedCommandSpec(r.Command); spec.AllowsMediaIO && r.Sectors == 0 {
		return fmt.Errorf("validate command request: command %q requires sectors", r.Command)
	}
	return nil
}

type CommandResult struct {
	ID     uint64
	Status string
	Data   []byte
}
