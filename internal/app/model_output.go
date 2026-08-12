package app

import (
	"discrescue/internal/device"
	"discrescue/internal/platform"
)

type OutputField uint8

const (
	OutputFieldDirectory OutputField = iota
	OutputFieldFileName
)

type JobSetupModel struct {
	ActionLabel       string
	OutputPath        string
	OutputDirectory   string
	OutputFileName    string
	ActiveOutputField OutputField
	OutputEditing     bool
	DefaultPath       string
	OutputFormat      string
	FreeSpace         string
	MethodLabel       string
	Method            platform.RecoveryMethod
	RetryUnresolved   bool
	ReadSpeed         device.ReadSpeedRequest
	CopyLabel         string
	ResumeReady       bool
	ResumeMapPath     string
	ResumeDetail      string
}
