package slash

type CompactResponseMode uint8

const (
	CompactResponseModeSummary CompactResponseMode = iota
	CompactResponseModeAck
)
