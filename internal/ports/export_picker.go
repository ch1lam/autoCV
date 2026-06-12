package ports

type ExportPicker interface {
	PickPDF(defaultName string) (string, bool, error)
	PickMarkdown(defaultName string) (string, bool, error)
}
