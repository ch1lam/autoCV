package wailsdialog

import (
	"fmt"

	"github.com/ch1lam/autocv/internal/ports"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ExportPicker struct {
	app *application.App
}

func NewExportPicker(app *application.App) *ExportPicker {
	return &ExportPicker{app: app}
}

func (picker *ExportPicker) PickPDF(
	defaultName string,
) (string, bool, error) {
	path, err := picker.app.Dialog.SaveFileWithOptions(
		&application.SaveFileDialogOptions{
			Title:    "导出 PDF 简历",
			Filename: defaultName,
		},
	).
		AddFilter("PDF", "*.pdf").
		PromptForSingleSelection()
	if err != nil {
		return "", false, fmt.Errorf("open PDF export dialog: %w", err)
	}
	return path, path != "", nil
}

func (picker *ExportPicker) PickMarkdown(
	defaultName string,
) (string, bool, error) {
	path, err := picker.app.Dialog.SaveFileWithOptions(
		&application.SaveFileDialogOptions{
			Title:    "导出 Markdown 简历",
			Filename: defaultName,
		},
	).
		AddFilter("Markdown", "*.md").
		PromptForSingleSelection()
	if err != nil {
		return "", false, fmt.Errorf(
			"open Markdown export dialog: %w",
			err,
		)
	}
	return path, path != "", nil
}

var _ ports.ExportPicker = (*ExportPicker)(nil)
