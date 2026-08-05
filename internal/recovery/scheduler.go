package recovery

type Strategy string

const (
	StrategyFast     Strategy = "fast"
	StrategyTrim     Strategy = "trim"
	StrategyAdaptive Strategy = "adaptive"
	StrategyScrape   Strategy = "scrape"
	StrategyVerify   Strategy = "verify"
)

type Request struct {
	StartLBA uint64
	Sectors  uint32
	Strategy Strategy
}
