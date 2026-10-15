// Package unsupported provides embeddable implementations of the parts of the
// ui contract that a backend has not implemented yet.
//
// Methods with an error result return an error wrapping ui.ErrNotSupported.
// Methods without one panic with such an error, so a missing feature fails
// loudly instead of silently doing nothing. Getters return zero values. A
// backend embeds these types and overrides methods as it implements them.
package unsupported

import (
	"fmt"

	"github.com/bluescreen10/gamekit/ui"
)

// Error returns an error that wraps ui.ErrNotSupported and names feature.
func Error(feature string) error {
	return fmt.Errorf("%s: %w", feature, ui.ErrNotSupported)
}

// Panic panics with Error(feature).
func Panic(feature string) {
	panic(Error(feature))
}
