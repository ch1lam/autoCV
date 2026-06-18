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

type DOCXPicker struct {
	app *application.App
}

func NewDOCXPicker(app *application.App) *DOCXPicker {
	return &DOCXPicker{app: app}
}

func (picker *DOCXPicker) PickDOCX() (
	ports.SelectedDOCX,
	bool,
	error,
) {
	path, err := picker.app.Dialog.OpenFile().
		SetTitle("导入 DOCX 职业资料").
		AddFilter("DOCX", "*.docx").
		PromptForSingleSelection()
	if err != nil {
		return ports.SelectedDOCX{}, false, fmt.Errorf(
			"open DOCX file dialog: %w",
			err,
		)
	}
	if path == "" {
		return ports.SelectedDOCX{}, false, nil
	}

	if strings.ToLower(filepath.Ext(path)) != ".docx" {
		return ports.SelectedDOCX{}, false, errors.New(
			"selected file is not DOCX",
		)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return ports.SelectedDOCX{}, false, fmt.Errorf(
			"read selected DOCX: %w",
			err,
		)
	}
	return ports.SelectedDOCX{
		OriginalName: filepath.Base(path),
		Contents:     contents,
	}, true, nil
}

var _ ports.DOCXPicker = (*DOCXPicker)(nil)
