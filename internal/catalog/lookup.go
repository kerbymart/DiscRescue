package catalog

func Lookup(entries []Entry, identity ContentIdentity) (Entry, bool) {
	for _, entry := range entries {
		if entry.Identity == identity {
			return entry, true
		}
	}
	return Entry{}, false
}
