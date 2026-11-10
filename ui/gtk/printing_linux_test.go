//go:build linux && cgo

package gtk_test

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

// TestMain runs the native operations on GTK's main thread. Test functions only
// inspect the result, like the other native GTK integration checks.
func checkGTKPrinting() error {
	directory, err := os.MkdirTemp("", "gamekit-gtk-print-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "pages.pdf")
	var rendered []int
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	copy(picture.Pix, []byte{255, 0, 0, 128, 0, 255, 0, 255, 0, 0, 255, 64, 255, 255, 255, 255})
	pages, err := gtk.ExportPrint(ui.PrintOptions{
		PageCount: func() int { return 2 },
		RenderPage: func(page int, canvas ui.PrintCanvas) error {
			rendered = append(rendered, page)
			if canvas.Size().Width <= 0 || canvas.Size().Height <= 0 {
				return fmt.Errorf("print canvas has invalid size: %v", canvas.Size())
			}
			if err := canvas.DrawText("GTK print bindings", ui.Point{X: 12, Y: 24}, ui.Font{Size: 12}); err != nil {
				return err
			}
			return canvas.DrawImage(picture, ui.Rect{Position: ui.Point{X: 12, Y: 36}, Size: ui.Size{Width: 32, Height: 32}})
		},
	}, path)
	if err != nil {
		return err
	}
	if pages != 2 || !slices.Equal(rendered, []int{0, 1}) {
		return fmt.Errorf("exported %d pages, rendered %v", pages, rendered)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return errors.New("GTK print export did not produce a PDF")
	}
	renderError := errors.New("render page failed")
	_, err = gtk.ExportPrint(ui.PrintOptions{
		PageCount:  func() int { return 1 },
		RenderPage: func(int, ui.PrintCanvas) error { return renderError },
	}, filepath.Join(directory, "cancelled.pdf"))
	if !errors.Is(err, renderError) {
		return fmt.Errorf("print error = %v, want renderer error", err)
	}
	return nil
}

func TestGTKPrintCallbacks(t *testing.T) {
	if gtkWidgets.err != nil {
		t.Fatal(gtkWidgets.err)
	}
	if gtkWidgets.printingErr != nil {
		t.Fatal(gtkWidgets.printingErr)
	}
}
