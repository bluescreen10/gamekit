//go:build linux && cgo

package gtk

import (
	"errors"
	"image"
	"runtime/cgo"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func runNativePrint(job *printJob, parent *gtkWindow) int {
	operation := gtkPrintOperationNew()
	defer gObjectUnref(unsafe.Pointer(operation))
	title := job.options.Title
	if title == "" {
		title = "GameKit"
	}
	gtkPrintOperationSetJobName(operation, stringPointer(title))
	gtkPrintOperationSetUnit(operation, gtkUnitPoints)
	gtkPrintOperationSetUseFullPage(operation, 0)
	action := gtkPrintOperationAction(gtkPrintOperationActionPrint)
	if job.options.ShowDialog {
		action = gtkPrintOperationActionPrintDialog
	}
	if parent != nil && gtkWidgetGetVisible((*gtkWidget)(unsafe.Pointer(parent))) == 0 {
		parent = nil
	}
	return executeNativePrint(job, operation, action, parent)
}

func executeNativePrint(job *printJob, operation *gtkPrintOperation, action gtkPrintOperationAction, parent *gtkWindow) int {
	handle := cgo.NewHandle(job)
	defer handle.Delete()
	// GTK retains signal data while the operation runs. Store only an integer
	// handle in native memory, and disconnect signals before freeing it.
	data := gMalloc(gSize(unsafe.Sizeof(uintptr(0))))
	*(*uintptr)(data) = uintptr(handle)
	defer gFree(data)
	begin := gSignalConnectData(unsafe.Pointer(operation), stringPointer("begin-print"), gtkPrintBeginCallback(), data, nil, 0)
	defer gSignalHandlerDisconnect(unsafe.Pointer(operation), begin)
	draw := gSignalConnectData(unsafe.Pointer(operation), stringPointer("draw-page"), gtkPrintDrawCallback(), data, nil, 0)
	defer gSignalHandlerDisconnect(unsafe.Pointer(operation), draw)
	var err *gError
	result := gtkPrintOperationRun(operation, action, parent, &err)
	if err != nil {
		gErrorFree(err)
	}
	if result != gtkPrintOperationResultApply {
		return -1
	}
	return max(1, job.pages)
}

func nativePrintJob(data unsafe.Pointer) *printJob {
	return cgo.Handle(*(*uintptr)(data)).Value().(*printJob)
}

func beginNativePrint(operation *gtkPrintOperation, _ *gtkPrintContext, data unsafe.Pointer) {
	job := nativePrintJob(data)
	job.pages = max(0, job.options.PageCount())
	gtkPrintOperationSetNPages(operation, cInt(max(1, job.pages)))
}

func drawNativePrint(operation *gtkPrintOperation, context *gtkPrintContext, page int, data unsafe.Pointer) {
	job := nativePrintJob(data)
	if job.err != nil {
		gtkPrintOperationCancel(operation)
		return
	}
	job.context = gtkPrintContextGetCairoContext(context)
	defer func() { job.context = nil }()
	canvas := printCanvas{
		size: ui.Size{Width: int(gtkPrintContextGetWidth(context)), Height: int(gtkPrintContextGetHeight(context))},
		job:  job,
	}
	if err := job.options.RenderPage(page, canvas); err != nil {
		job.err = err
		gtkPrintOperationCancel(operation)
	}
}

// A print canvas borrows the job's Cairo context only during RenderPage.
// Clearing the job context also invalidates any copy retained by a caller.
type printCanvas struct {
	size ui.Size
	job  *printJob
}

func (p printCanvas) Size() ui.Size {
	return p.size
}

func (p printCanvas) DrawImage(source image.Image, bounds ui.Rect) error {
	if source == nil || source.Bounds().Empty() {
		return errors.New("ui: print image is empty")
	}
	if p.job == nil || p.job.context == nil {
		return nil
	}
	pixels, width, height := rgbaPixels(source)
	surface := cairoImageSurfaceCreate(cairoFormatARGB32, cInt(width), cInt(height))
	defer cairoSurfaceDestroy(surface)
	stride := int(cairoImageSurfaceGetStride(surface)) / 4
	data := cairoImageSurfaceGetData(surface)
	target := unsafe.Slice((*uint32)(unsafe.Pointer(data)), stride*height)
	for y := range height {
		for x := range width {
			i := (y*width + x) * 4
			alpha := uint32(pixels[i+3])
			red := uint32(pixels[i]) * alpha / 255
			green := uint32(pixels[i+1]) * alpha / 255
			blue := uint32(pixels[i+2]) * alpha / 255
			target[y*stride+x] = alpha<<24 | red<<16 | green<<8 | blue
		}
	}
	cairoSurfaceMarkDirty(surface)
	context := p.job.context
	cairoSave(context)
	cairoTranslate(context, cDouble(bounds.Position.X), cDouble(bounds.Position.Y))
	cairoScale(context, cDouble(bounds.Size.Width)/cDouble(width), cDouble(bounds.Size.Height)/cDouble(height))
	cairoSetSourceSurface(context, surface, 0, 0)
	cairoPaint(context)
	cairoRestore(context)
	return nil
}

func (p printCanvas) DrawText(text string, position ui.Point, font ui.Font) error {
	if err := font.Validate(); err != nil {
		return err
	}
	if !nativeFontFamilyInstalled(font.Family) {
		return errors.New("ui/gtk: font family is not installed")
	}
	if p.job == nil || p.job.context == nil {
		return nil
	}
	context := p.job.context
	layout := pangoCairoCreateLayout(context)
	defer gObjectUnref(unsafe.Pointer(layout))
	description := pangoFontDescriptionNew()
	defer pangoFontDescriptionFree(description)
	family := font.Family
	if family == "" || family == "System" {
		family = "Sans"
	}
	size := font.Size
	if size <= 0 {
		size = 12
	}
	weight := pangoWeight(font.Weight)
	if weight == 0 {
		weight = pangoWeightNormal
	}
	style := pangoStyle(pangoStyleNormal)
	if font.Italic {
		style = pangoStyleItalic
	}
	pangoFontDescriptionSetFamily(description, stringPointer(family))
	pangoFontDescriptionSetSize(description, cInt(size*pangoScale))
	pangoFontDescriptionSetWeight(description, weight)
	pangoFontDescriptionSetStyle(description, style)
	pangoLayoutSetFontDescription(layout, description)
	pangoLayoutSetText(layout, stringPointer(text), -1)
	cairoMoveTo(context, cDouble(position.X), cDouble(position.Y))
	cairoSetSourceRGB(context, 0, 0, 0)
	pangoCairoShowLayout(context, layout)
	return nil
}

func nativeFontFamilyInstalled(family string) bool {
	if family == "" || family == "System" || family == "Monospace" {
		return true
	}
	var families **pangoFontFamily
	var count cInt
	pangoFontMapListFamilies(pangoCairoFontMapGetDefault(), &families, &count)
	defer gFree(unsafe.Pointer(families))
	name := stringPointer(family)
	for _, candidate := range unsafe.Slice(families, int(count)) {
		if gASCIIStrcasecmp(pangoFontFamilyGetName(candidate), name) == 0 {
			return true
		}
	}
	return false
}
