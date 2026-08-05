package recovery

func AdaptivePass(startLBA uint64, sectors uint32) Request {
	return Request{StartLBA: startLBA, Sectors: sectors, Strategy: StrategyAdaptive}
}
