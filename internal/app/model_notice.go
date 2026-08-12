package app

type DetailsViewModel struct {
	Lines []string
}

type DialogModel struct {
	Title   string
	Body    string
	Options []string
	Cursor  int
}

type NoticeModel struct {
	Code            UserMessageCode
	Text            string
	Explanation     string
	Action          string
	TechnicalDetail string
	Severity        Severity
}
