package wailsdialog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ch1lam/autocv/internal/ports"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type SourcePDFPicker struct {
	app *application.App
}

func NewSourcePDFPicker(app *application.App) *SourcePDFPicker {
	return &SourcePDFPicker{app: app}
}

func (picker *SourcePDFPicker) PickSourcePDF() (
	ports.SelectedPDF,
	bool,
	error,
) {
	path, err := picker.app.Dialog.OpenFile().
		SetTitle("导入文本型 PDF 职业资料").
		AddFilter("PDF", "*.pdf").
		PromptForSingleSelection()
	if err != nil {
		return ports.SelectedPDF{}, false, fmt.Errorf(
			"open PDF file dialog: %w",
			err,
		)
	}
	if path == "" {
		return ports.SelectedPDF{}, false, nil
	}

	if strings.ToLower(filepath.Ext(path)) != ".pdf" {
		return ports.SelectedPDF{}, false, errors.New(
			"selected file is not PDF",
		)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return ports.SelectedPDF{}, false, fmt.Errorf(
			"read selected PDF: %w",
			err,
		)
	}
	return ports.SelectedPDF{
		OriginalName: filepath.Base(path),
		Contents:     contents,
	}, true, nil
}

var _ ports.PDFPicker = (*SourcePDFPicker)(nil)
