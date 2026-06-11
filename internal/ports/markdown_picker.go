package ports

type SelectedMarkdown struct {
	OriginalName string
	Contents     []byte
}

type MarkdownPicker interface {
	PickMarkdown() (SelectedMarkdown, bool, error)
}
