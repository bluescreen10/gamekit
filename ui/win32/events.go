//go:build windows

package win32

import (
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

// The native layer reports events to the Go layer as values of the types below,
// and the receiver selects them with a type switch. Events that the ui package
// defines, such as ui.PointerMoveEvent, ui.KeyEvent, and ui.SurfaceEvent, go as
// they are.

// eventReceiver receives the events of a native object.
type eventReceiver interface {
	receiveEvent(event any)
}

// queryReceiver answers the questions of a native object, such as whether a
// window can close. A native object without a queryReceiver takes yes as the
// answer.
type queryReceiver interface {
	answerQuery(query any) bool
}

// Events of views.
type (
	// focusEvent reports that a view or a window got the keyboard focus.
	focusEvent struct{}
	// blurEvent reports that a view or a window lost the keyboard focus.
	blurEvent struct{}
	// clickEvent reports that the user clicked a button or a link.
	clickEvent struct{}
	// changeEvent reports that the user changed the state of a control: its
	// value, its selection, its checked state, its selected tab, or its
	// expansion. The receiver reads the new state from the view.
	changeEvent struct{}
	// textChangeEvent reports that the user changed the text of a control.
	// An empty text tells the receiver to read the text from the view.
	textChangeEvent struct {
		text string
	}
	// submitEvent reports that the user committed the text of a control.
	submitEvent struct{}
	// actionEvent reports that the user chose an action.
	actionEvent struct {
		id string
	}
	// activateEvent reports that the user activated an item, by its
	// identifier or by its index.
	activateEvent struct {
		id    string
		index int
	}
	// sortEvent reports that the user sorted a table by a column.
	sortEvent struct {
		column    string
		ascending bool
	}
	// expandEvent reports that the user expanded or collapsed an item, or
	// revealed or hid a password.
	expandEvent struct {
		id       string
		expanded bool
	}
	// closeEvent reports that the user closed a view, a popover, or a dialog.
	closeEvent struct{}
	// layoutChangeEvent reports that the user moved a divider.
	layoutChangeEvent struct{}
	// rangeChangeEvent reports the new range of a range slider.
	rangeChangeEvent struct {
		lower, upper float64
	}
	// pageChangeEvent reports the page that the user selected.
	pageChangeEvent struct {
		page int
	}
	// keyRecordEvent reports the key that a key recorder recorded: the
	// modifier bits, the key code, and the text of the key.
	keyRecordEvent struct {
		modifiers int
		key       int
		label     string
	}
	// charEvent reports a character that the keyboard layout produced.
	charEvent struct {
		char rune
	}
	// frameEvent asks a canvas for a frame.
	frameEvent struct {
		interval time.Duration
	}
)

// Events of windows and the application.
type (
	windowResizeEvent struct {
		size ui.Size
	}
	fullscreenEvent struct {
		fullscreen bool
	}
	themeEvent struct {
		theme ui.Theme
	}
	fileDropEvent struct {
		paths []string
	}
	pointerEnterEvent struct {
		crossing ui.PointerCrossingEvent
	}
	pointerLeaveEvent struct {
		crossing ui.PointerCrossingEvent
	}
	occlusionEvent struct {
		occluded bool
	}
	// appActivationEvent reports that the application became active or
	// inactive.
	appActivationEvent struct {
		active bool
	}
	notificationEvent struct {
		activation ui.NotificationActivation
	}
)

// Queries.
type (
	closeRequest struct{}
	quitRequest  struct{}
	// linkRequest asks whether the application opens a link itself. For no,
	// the backend opens it with the default program.
	linkRequest struct {
		url string
	}
)

// send reports an event to a receiver, if there is one.
func send(receiver eventReceiver, event any) {
	if receiver != nil {
		receiver.receiveEvent(event)
	}
}

// ask asks a receiver a question. Without a receiver that answers queries, the
// answer is yes.
func ask(receiver eventReceiver, query any) bool {
	if answerer, ok := receiver.(queryReceiver); ok {
		return answerer.answerQuery(query)
	}
	return true
}
