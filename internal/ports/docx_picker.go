package ports

type SelectedDOCX struct {
	OriginalName string
	Contents     []byte
}

type DOCXPicker interface {
	PickDOCX() (SelectedDOCX, bool, error)
}
