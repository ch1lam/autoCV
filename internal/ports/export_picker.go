package ports

type ExportPicker interface {
	PickPDF(defaultName string) (string, bool, error)
	PickMarkdown(defaultName string) (string, bool, error)
}

type ProfileExportPicker interface {
	PickProfileJSON(defaultName string) (string, bool, error)
}
