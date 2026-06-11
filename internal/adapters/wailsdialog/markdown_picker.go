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

type MarkdownPicker struct {
	app *application.App
}

func NewMarkdownPicker(app *application.App) *MarkdownPicker {
	return &MarkdownPicker{app: app}
}

func (picker *MarkdownPicker) PickMarkdown() (
	ports.SelectedMarkdown,
	bool,
	error,
) {
	path, err := picker.app.Dialog.OpenFile().
		SetTitle("导入 Markdown 职业资料").
		AddFilter("Markdown", "*.md;*.markdown").
		PromptForSingleSelection()
	if err != nil {
		return ports.SelectedMarkdown{}, false, fmt.Errorf(
			"open Markdown file dialog: %w",
			err,
		)
	}
	if path == "" {
		return ports.SelectedMarkdown{}, false, nil
	}

	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".md" && extension != ".markdown" {
		return ports.SelectedMarkdown{}, false, errors.New(
			"selected file is not Markdown",
		)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return ports.SelectedMarkdown{}, false, fmt.Errorf(
			"read selected Markdown: %w",
			err,
		)
	}
	return ports.SelectedMarkdown{
		OriginalName: filepath.Base(path),
		Contents:     contents,
	}, true, nil
}

var _ ports.MarkdownPicker = (*MarkdownPicker)(nil)
