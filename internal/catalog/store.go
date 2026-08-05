package catalog

type Store struct {
	Entries []Entry
}

type Entry struct {
	Identity ContentIdentity
	Status   string
}
