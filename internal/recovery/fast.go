package recovery

func FastPass(startLBA uint64, sectors uint32) Request {
	return Request{StartLBA: startLBA, Sectors: sectors, Strategy: StrategyFast}
}
