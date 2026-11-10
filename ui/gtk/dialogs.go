//go:build linux && cgo

package gtk

import (
	"context"
	"strings"

	"github.com/bluescreen10/gamekit/ui"
)

// OpenFile displays a native open-file panel parented to the window.
func (w *Window) OpenFile(ctx context.Context, options ui.OpenFileOptions) (string, error) {
	paths, err := w.openPanel(ctx, options, false, false)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// OpenFiles displays a native multiple-file panel parented to the window.
func (w *Window) OpenFiles(ctx context.Context, options ui.OpenFileOptions) ([]string, error) {
	return w.openPanel(ctx, options, true, false)
}

// SaveFile displays a native save-file panel parented to the window.
func (w *Window) SaveFile(ctx context.Context, options ui.SaveFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	filename := options.Filename
	if filename != "" && options.DefaultExtension != "" && !strings.Contains(filename, ".") {
		filename += "." + strings.TrimPrefix(options.DefaultExtension, ".")
	}
	dialog := newFileDialog(options.Title, options.Directory, filename, options.Filters)
	paths, err := w.runFilePanel(ctx, dialog, saveFilePanel)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// SelectDirectory displays a native directory panel parented to the window.
func (w *Window) SelectDirectory(ctx context.Context, options ui.DirectoryDialogOptions) (string, error) {
	paths, err := w.openPanel(ctx, ui.OpenFileOptions{
		Title:     options.Title,
		Directory: options.Directory,
	}, false, true)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *Window) openPanel(ctx context.Context, options ui.OpenFileOptions, multiple, directories bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	mode := openFilePanel
	filters := options.Filters
	if directories {
		mode = selectFolderPanel
		filters = nil
	} else if multiple {
		mode = openFilesPanel
	}
	return w.runFilePanel(ctx, newFileDialog(options.Title, options.Directory, options.Filename, filters), mode)
}

func fileExtensions(filters []ui.FileFilter) string {
	extensions := make([]string, 0)
	for _, filter := range filters {
		for _, pattern := range filter.Patterns {
			extension := strings.TrimPrefix(strings.TrimSpace(pattern), "*.")
			if extension != "" && extension != "*" {
				extensions = append(extensions, extension)
			}
		}
	}
	return strings.Join(extensions, ",")
}
