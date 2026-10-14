// Package ui defines a backend-neutral native user-interface protocol.
//
// Backend creates windows. Each Window owns its controls, dialogs, event
// dispatch, and presentation surfaces. Import ui/default to register the
// platform default, or import a backend package and select it with New. Window
// and Canvas both implement Drawable. A GPU backend can create a presentation
// surface for a whole window or for a region beside native controls.
package ui
