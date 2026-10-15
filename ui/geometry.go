package ui

// Point is a position in logical pixels relative to a widget's parent.
type Point struct {
	// X is the distance from the left edge. It increases to the right.
	X int
	// Y is the distance from the top edge. It increases downward.
	Y int
}

// Size is a width and height in logical pixels.
type Size struct {
	// Width is the horizontal dimension.
	Width int
	// Height is the vertical dimension.
	Height int
}

// IsValid reports whether both dimensions are positive.
func (s Size) IsValid() bool {
	return s.Width > 0 && s.Height > 0
}

// Rect is a widget's position and size in logical pixels.
type Rect struct {
	// Position is the top-left corner.
	Position Point
	// Size is the width and height.
	Size Size
}
