package recovery

func ScrapePass(startLBA uint64, sectors uint32) Request {
	return Request{StartLBA: startLBA, Sectors: sectors, Strategy: StrategyScrape}
}
