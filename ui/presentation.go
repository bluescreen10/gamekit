package ui

import (
	"errors"
	stdimage "image"
	"image/color"
	"math"
)

// IconSource names a platform symbol or supplies an image. SystemName uses a
// backend-neutral name such as "trash", "gear", or "folder"; backends map it to
// the nearest native symbol. Image is used when SystemName is empty or has no
// native equivalent.
type IconSource struct {
	// SystemName is the backend-neutral name of a symbol.
	SystemName string

	// Image is the image to show when there is no symbol.
	Image stdimage.Image
}

// IsZero reports whether the source names no icon.
func (source IconSource) IsZero() bool {
	return source.SystemName == "" && source.Image == nil
}

// IconPlacement controls where a control draws its icon relative to its text.
type IconPlacement uint8

const (
	// IconBeforeText draws the icon before the text.
	IconBeforeText IconPlacement = iota

	// IconAfterText draws the icon after the text.
	IconAfterText

	// IconAboveText draws the icon above the text.
	IconAboveText

	// IconBelowText draws the icon below the text.
	IconBelowText

	// IconOnly draws the icon without the text.
	IconOnly
)

// FontWeight is a CSS-style font weight from 100 through 900. Zero selects the
// regular weight.
type FontWeight uint16

const (
	// FontWeightRegular is the normal weight of body text.
	FontWeightRegular FontWeight = 400

	// FontWeightMedium is a weight between regular and bold.
	FontWeightMedium FontWeight = 500

	// FontWeightBold is the bold weight.
	FontWeightBold FontWeight = 700
)

// Font describes a font request. Family "System" selects the platform UI font.
// Size is in logical points; zero selects the platform default size.
type Font struct {
	// Family is the name of the font family.
	Family string

	// Size is the size of the font.
	Size float64

	// Weight is the thickness of the strokes.
	Weight FontWeight

	// Italic selects the italic style.
	Italic bool
}

// Validate checks that the font request is usable.
func (font Font) Validate() error {
	if math.IsNaN(font.Size) || math.IsInf(font.Size, 0) || font.Size < 0 {
		return errors.New("ui: font size must be finite and non-negative")
	}
	if font.Weight != 0 && (font.Weight < 100 || font.Weight > 900) {
		return errors.New("ui: font weight must be between 100 and 900")
	}
	return nil
}

// SemanticColor names a theme-aware color role. Backends resolve it to the
// native color for the current theme, contrast, and accent settings.
type SemanticColor uint8

const (
	// SemanticColorDefault keeps the native color of the widget.
	SemanticColorDefault SemanticColor = iota

	// SemanticColorPrimary is the color of primary text.
	SemanticColorPrimary

	// SemanticColorSecondary is the color of less important text.
	SemanticColorSecondary

	// SemanticColorTertiary is the color of the least important text.
	SemanticColorTertiary

	// SemanticColorSurface is the color of a background surface.
	SemanticColorSurface

	// SemanticColorAccent is the accent color that the user selects.
	SemanticColorAccent

	// SemanticColorSuccess is the color that shows success.
	SemanticColorSuccess

	// SemanticColorWarning is the color that shows a warning.
	SemanticColorWarning

	// SemanticColorError is the color that shows an error.
	SemanticColorError

	// SemanticColorLink is the color of links.
	SemanticColorLink
)

// WidgetStyle overrides a widget's native font and colors. Zero fields keep the
// native default.
type WidgetStyle struct {
	// Font is the font of the text. The zero value keeps the native font.
	Font Font

	// Foreground is the color of the text.
	Foreground SemanticColor

	// Background is the color behind the widget.
	Background SemanticColor

	// ForegroundColor is an exact color of the text. It replaces Foreground.
	// Nil uses Foreground.
	ForegroundColor color.Color

	// BackgroundColor is an exact color behind the widget. It replaces
	// Background. Nil uses Background. A button with a background color
	// becomes a flat button filled with that color.
	BackgroundColor color.Color

	// CornerRadius rounds the corners of the background, in logical pixels.
	CornerRadius int
}

// Validate checks that the style uses recognized values.
func (style WidgetStyle) Validate() error {
	if err := style.Font.Validate(); err != nil {
		return err
	}
	if style.CornerRadius < 0 {
		return errors.New("ui: corner radius must be non-negative")
	}
	if style.Foreground > SemanticColorLink || style.Background > SemanticColorLink {
		return errors.New("ui: invalid semantic color")
	}
	return nil
}

// Cursor is the pointer shape shown while the pointer is over a widget.
type Cursor uint8

const (
	// CursorDefault is the arrow.
	CursorDefault Cursor = iota

	// CursorPointer is the hand that shows a link or a button.
	CursorPointer

	// CursorText is the I-beam that shows editable text.
	CursorText

	// CursorCrosshair is a crosshair for precise selection.
	CursorCrosshair

	// CursorMove shows that the user can move an object.
	CursorMove

	// CursorResizeHorizontal shows that the user can resize to the left or right.
	CursorResizeHorizontal

	// CursorResizeVertical shows that the user can resize up or down.
	CursorResizeVertical

	// CursorNotAllowed shows that the action is not available.
	CursorNotAllowed

	// CursorWait shows that the application is busy.
	CursorWait

	// CursorGrab is an open hand that shows that the user can drag.
	CursorGrab

	// CursorGrabbing is a closed hand during a drag.
	CursorGrabbing
)

// Validate checks that the cursor is recognized.
func (cursor Cursor) Validate() error {
	if cursor > CursorGrabbing {
		return errors.New("ui: invalid cursor")
	}
	return nil
}

// Severity classifies a message.
type Severity uint8

const (
	// SeverityInformation is neutral information.
	SeverityInformation Severity = iota

	// SeveritySuccess reports a successful result.
	SeveritySuccess

	// SeverityWarning reports a possible problem.
	SeverityWarning

	// SeverityError reports a failure.
	SeverityError
)

// Validate checks that the severity is recognized.
func (severity Severity) Validate() error {
	if severity > SeverityError {
		return errors.New("ui: invalid severity")
	}
	return nil
}

// ControlSize selects one of the platform's standard control sizes.
type ControlSize uint8

const (
	// ControlSizeRegular is the default size.
	ControlSizeRegular ControlSize = iota

	// ControlSizeMini is the smallest size.
	ControlSizeMini

	// ControlSizeSmall is smaller than regular.
	ControlSizeSmall

	// ControlSizeLarge is larger than regular.
	ControlSizeLarge
)

// Anchor positions content inside a larger area.
type Anchor uint8

const (
	// AnchorCenter is the middle of the area.
	AnchorCenter Anchor = iota

	// AnchorTopLeft is the top-left corner.
	AnchorTopLeft

	// AnchorTop is the middle of the top edge.
	AnchorTop

	// AnchorTopRight is the top-right corner.
	AnchorTopRight

	// AnchorLeft is the middle of the left edge.
	AnchorLeft

	// AnchorRight is the middle of the right edge.
	AnchorRight

	// AnchorBottomLeft is the bottom-left corner.
	AnchorBottomLeft

	// AnchorBottom is the middle of the bottom edge.
	AnchorBottom

	// AnchorBottomRight is the bottom-right corner.
	AnchorBottomRight
)

// Edge names one side of a rectangle.
type Edge uint8

const (
	// EdgeTop is the top side.
	EdgeTop Edge = iota

	// EdgeBottom is the bottom side.
	EdgeBottom

	// EdgeLeft is the left side.
	EdgeLeft

	// EdgeRight is the right side.
	EdgeRight
)

// SelectionMode controls how many items can be selected.
type SelectionMode uint8

const (
	// SelectionNone allows no selection.
	SelectionNone SelectionMode = iota

	// SelectionSingle allows one selected item.
	SelectionSingle

	// SelectionMultiple allows many selected items.
	SelectionMultiple
)

// TextAlignment aligns text within its control. Start and End follow the text
// direction.
type TextAlignment uint8

const (
	// TextAlignStart aligns text to the start of the line.
	TextAlignStart TextAlignment = iota

	// TextAlignCenter centers text.
	TextAlignCenter

	// TextAlignEnd aligns text to the end of the line.
	TextAlignEnd

	// TextAlignJustified aligns text to both edges.
	TextAlignJustified
)

// WrapMode controls how text wraps when it is wider than its control.
type WrapMode uint8

const (
	// WrapNone keeps text on one line.
	WrapNone WrapMode = iota

	// WrapWords breaks lines between words.
	WrapWords

	// WrapCharacters breaks lines between characters.
	WrapCharacters
)

// Truncation controls where text that does not fit is elided.
type Truncation uint8

const (
	// TruncateNone clips text that does not fit.
	TruncateNone Truncation = iota

	// TruncateHead replaces the start of the text with an ellipsis.
	TruncateHead

	// TruncateMiddle replaces the middle of the text with an ellipsis.
	TruncateMiddle

	// TruncateTail replaces the end of the text with an ellipsis.
	TruncateTail
)

// TextStyle is a semantic text style that follows the platform type ramp and
// the user's text-size setting.
type TextStyle uint8

const (
	// TextStyleBody is the style of normal text.
	TextStyleBody TextStyle = iota

	// TextStyleCaption is the style of small secondary text.
	TextStyleCaption

	// TextStyleHeading is the style of a section heading.
	TextStyleHeading

	// TextStyleTitle is the style of a page or window title.
	TextStyleTitle

	// TextStyleMonospace is a style in which all characters have the same width.
	TextStyleMonospace
)

// TextRange is a half-open range of rune offsets.
type TextRange struct {
	// Start is the offset of the first rune in the range.
	Start int

	// End is the offset after the last rune in the range.
	End int
}

// ColorSpace is the color space of a GPU surface.
type ColorSpace uint8

const (
	// ColorSpaceUnknown means that the backend cannot tell the color space.
	ColorSpaceUnknown ColorSpace = iota

	// ColorSpaceSRGB is the standard sRGB color space.
	ColorSpaceSRGB

	// ColorSpaceDisplayP3 is the Display P3 wide-gamut color space.
	ColorSpaceDisplayP3

	// ColorSpaceExtendedSRGB is sRGB with values outside the range from zero to
	// one.
	ColorSpaceExtendedSRGB
)
