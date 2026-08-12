package app

type StartRequested struct{}

type QuitRequested struct{}

type CatalogUpdatedMsg struct {
	ContentID [32]byte
	Err       error
}
