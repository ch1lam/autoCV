package ports

type SelectedPDF struct {
	OriginalName string
	Contents     []byte
}

type PDFPicker interface {
	PickSourcePDF() (SelectedPDF, bool, error)
}
