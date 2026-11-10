//go:build linux && cgo

package gtk

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unsafe"
)

type nativeDateControl struct {
	widget, calendar, button, hour, minute, second, meridiem *gtkWidget
	value, minimum, maximum                                  float64
	suppress                                                 bool
}

func nativeCalendarDate(calendar *gtkWidget) time.Time {
	date := gtkCalendarGetDate((*gtkCalendar)(unsafe.Pointer(calendar)))
	defer gDateTimeUnref(date)
	return time.Date(int(gDateTimeGetYear(date)), time.Month(gDateTimeGetMonth(date)), int(gDateTimeGetDayOfMonth(date)), 0, 0, 0, 0, time.UTC)
}
func selectNativeCalendar(calendar *gtkWidget, value time.Time) {
	date := gDateTimeNewLocal(cInt(value.Year()), cInt(value.Month()), cInt(value.Day()), 12, 0, 0)
	gtkCalendarSelectDay((*gtkCalendar)(unsafe.Pointer(calendar)), date)
	gDateTimeUnref(date)
}
func (state *nativeDateControl) showDate() {
	state.suppress = true
	selectNativeCalendar(state.calendar, time.Unix(int64(state.value), 0).UTC())
	state.suppress = false
	if state.button != nil {
		date := gDateTimeNewFromUnixUtc(cInt64(state.value))
		text := takeString(gDateTimeFormat(date, stringPointer("%x")))
		gDateTimeUnref(date)
		gtkMenuButtonSetLabel((*gtkMenuButton)(unsafe.Pointer(state.button)), stringPointer(text))
		setAccessibleText(state.button, gtkAccessiblePropertyValueText, text)
	}
}
func (state *nativeDateControl) showTime() {
	state.suppress = true
	defer func() { state.suppress = false }()
	seconds := int(math.Mod(math.Mod(math.Floor(state.value), 86400)+86400, 86400))
	hour := seconds / 3600
	if state.meridiem != nil {
		display := hour % 12
		if display == 0 {
			display = 12
		}
		gtkSpinButtonSetValue((*gtkSpinButton)(unsafe.Pointer(state.hour)), cDouble(display))
		meridiem := cUint(0)
		if hour >= 12 {
			meridiem = 1
		}
		gtkDropDownSetSelected((*gtkDropDown)(unsafe.Pointer(state.meridiem)), meridiem)
	} else {
		gtkSpinButtonSetValue((*gtkSpinButton)(unsafe.Pointer(state.hour)), cDouble(hour))
	}
	gtkSpinButtonSetValue((*gtkSpinButton)(unsafe.Pointer(state.minute)), cDouble(seconds/60%60))
	if state.second != nil {
		gtkSpinButtonSetValue((*gtkSpinButton)(unsafe.Pointer(state.second)), cDouble(seconds%60))
	}
}
func newTimeSpin(minimum, maximum int, label string) *gtkWidget {
	widget := gtkSpinButtonNewWithRange(cDouble(minimum), cDouble(maximum), 1)
	spin := (*gtkSpinButton)(unsafe.Pointer(widget))
	gtkSpinButtonSetWrap(spin, 1)
	gtkSpinButtonSetNumeric(spin, 1)
	gtkOrientableSetOrientation((*gtkOrientable)(unsafe.Pointer(widget)), gtkOrientationVertical)
	setAccessibleText(widget, gtkAccessiblePropertyLabel, label)
	connectNativeArgument(unsafe.Pointer(widget), "output", gtkCloseRequestCallback(), func() bool {
		gtkEditableSetText((*gtkEditable)(unsafe.Pointer(widget)), stringPointer(fmt.Sprintf("%02d", gtkSpinButtonGetValueAsInt(spin))))
		return true
	})
	return widget
}
func localeUses12Hour() bool {
	date := gDateTimeNewUtc(2000, 1, 1, 13, 0, 0)
	defer gDateTimeUnref(date)
	return !strings.Contains(takeString(gDateTimeFormat(date, stringPointer("%X"))), "13")
}
func gkGTKDatePickerCreate(window cUintptr, style, elements cInt, value, minimum, maximum cDouble, rangeMode cInt, interval cDouble, clock cInt) unsafe.Pointer {
	state := &nativeDateControl{value: float64(value), minimum: float64(minimum), maximum: float64(maximum)}
	if elements&0x00e0 != 0 {
		state.calendar = gtkCalendarNew()
		state.widget = state.calendar
		if style != 1 {
			state.button = gtkMenuButtonNew()
			popover := gtkPopoverNew()
			gtkPopoverSetChild((*gtkPopover)(unsafe.Pointer(popover)), state.calendar)
			gtkMenuButtonSetPopover((*gtkMenuButton)(unsafe.Pointer(state.button)), popover)
			state.widget = state.button
		}
		registerNativeComponent(uintptr(window), state.widget, state.calendar, kindDate)
		storeComponentState(state.widget, state)
		state.showDate()
		connectNativeSignal(unsafe.Pointer(state.calendar), "day-selected", func() {
			if state.suppress {
				return
			}
			value := float64(nativeCalendarDate(state.calendar).Unix())
			if state.minimum != 0 {
				value = max(state.minimum, value)
			}
			if state.maximum != 0 {
				value = min(state.maximum, value)
			}
			state.value = value
			state.showDate()
			sendComponentEvent(state.widget, gtkEvent{kind: gkEventValue, x: value})
		})
		return unsafe.Pointer(state.widget)
	}
	state.widget = boxWithRole(gtkOrientationHorizontal, 2, gtkAccessibleRoleGroup)
	twelve := clock == 1 || clock == 0 && localeUses12Hour()
	minimumHour, maximumHour := 0, 23
	if twelve {
		minimumHour, maximumHour = 1, 12
	}
	state.hour, state.minute = newTimeSpin(minimumHour, maximumHour, "Hour"), newTimeSpin(0, 59, "Minute")
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), state.hour)
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), gtkLabelNew(stringPointer(":")))
	gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), state.minute)
	if elements&0x0002 != 0 {
		state.second = newTimeSpin(0, 59, "Second")
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), gtkLabelNew(stringPointer(":")))
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), state.second)
	}
	if twelve {
		names, free := nativeStringVector([]string{"AM", "PM"})
		state.meridiem = gtkDropDownNewFromStrings(names)
		free()
		gtkWidgetSetValign(state.meridiem, gtkAlignCenter)
		gtkBoxAppend((*gtkBox)(unsafe.Pointer(state.widget)), state.meridiem)
	}
	registerNativeComponent(uintptr(window), state.widget, state.hour, kindDate)
	storeComponentState(state.widget, state)
	state.showTime()
	changed := func() {
		if state.suppress {
			return
		}
		hour := int(gtkSpinButtonGetValueAsInt((*gtkSpinButton)(unsafe.Pointer(state.hour))))
		if state.meridiem != nil {
			hour %= 12
			if gtkDropDownGetSelected((*gtkDropDown)(unsafe.Pointer(state.meridiem))) == 1 {
				hour += 12
			}
		}
		seconds := hour*3600 + int(gtkSpinButtonGetValueAsInt((*gtkSpinButton)(unsafe.Pointer(state.minute))))*60
		if state.second != nil {
			seconds += int(gtkSpinButtonGetValueAsInt((*gtkSpinButton)(unsafe.Pointer(state.second))))
		}
		state.value = float64(seconds)
		sendComponentEvent(state.widget, gtkEvent{kind: gkEventValue, x: state.value})
	}
	for _, spin := range []*gtkWidget{state.hour, state.minute, state.second} {
		if spin != nil {
			connectNativeSignal(unsafe.Pointer(spin), "value-changed", changed)
		}
	}
	if state.meridiem != nil {
		connectNativeNotify(unsafe.Pointer(state.meridiem), "notify::selected", changed)
	}
	return unsafe.Pointer(state.widget)
}
func gkGTKDatePickerValue(pointer unsafe.Pointer, value, interval *cDouble) {
	*value, *interval = 0, 0
	if state := componentState[nativeDateControl](pointer); state != nil {
		*value = cDouble(state.value)
	}
}
func gkGTKDatePickerSet(pointer unsafe.Pointer, value, interval cDouble) {
	if state := componentState[nativeDateControl](pointer); state != nil {
		state.value = float64(value)
		if state.calendar != nil {
			state.showDate()
		} else {
			state.showTime()
		}
	}
}

type nativeCalendar struct {
	widget           *gtkWidget
	selection        int
	dates            []float64
	anchor, suppress bool
}

func (state *nativeCalendar) mark() {
	calendar := (*gtkCalendar)(unsafe.Pointer(state.widget))
	gtkCalendarClearMarks(calendar)
	shown := nativeCalendarDate(state.widget)
	first, last := 0.0, 0.0
	if len(state.dates) > 0 {
		first, last = state.dates[0], state.dates[len(state.dates)-1]
	}
	for day := 1; day <= 31; day++ {
		date := time.Date(shown.Year(), shown.Month(), day, 0, 0, 0, 0, time.UTC)
		if date.Month() != shown.Month() {
			break
		}
		seconds := float64(date.Unix())
		marked := false
		if state.selection == 2 {
			marked = len(state.dates) > 0 && seconds >= first && seconds <= last
		} else if state.selection == 1 {
			for _, selected := range state.dates {
				if math.Floor(selected/86400) == math.Floor(seconds/86400) {
					marked = true
					break
				}
			}
		}
		if marked {
			gtkCalendarMarkDay(calendar, cUint(day))
		}
	}
}
func (state *nativeCalendar) showMonth(value time.Time) {
	state.suppress = true
	selectNativeCalendar(state.widget, value)
	state.suppress = false
	state.mark()
}
func gkGTKCalendarCreate(window cUintptr, selection cInt, dates *cDouble, count cInt, visible cDouble) unsafe.Pointer {
	state := &nativeCalendar{widget: gtkCalendarNew(), selection: int(selection), anchor: true}
	registerNativeComponent(uintptr(window), state.widget, nil, kindCalendar)
	storeComponentState(state.widget, state)
	gtkCalendarSetShowDayNames((*gtkCalendar)(unsafe.Pointer(state.widget)), 1)
	gkGTKCalendarSetDates(unsafe.Pointer(state.widget), dates, count)
	if visible != 0 {
		gkGTKCalendarSetVisibleMonth(unsafe.Pointer(state.widget), visible)
	}
	connectNativeSignal(unsafe.Pointer(state.widget), "day-selected", func() {
		if state.suppress {
			return
		}
		seconds := float64(nativeCalendarDate(state.widget).Unix())
		switch state.selection {
		case 0:
			state.dates = []float64{seconds}
		case 2:
			if state.anchor || len(state.dates) == 0 {
				state.dates = []float64{seconds}
				state.anchor = false
			} else {
				state.dates = []float64{state.dates[0], seconds}
				slices.Sort(state.dates)
				state.anchor = true
			}
		default:
			index := slices.IndexFunc(state.dates, func(value float64) bool { return math.Floor(value/86400) == math.Floor(seconds/86400) })
			if index >= 0 {
				state.dates = append(state.dates[:index], state.dates[index+1:]...)
			} else {
				state.dates = append(state.dates, seconds)
				slices.Sort(state.dates)
			}
		}
		state.mark()
		sendComponentEvent(state.widget, gtkEvent{kind: gkEventValue, x: seconds})
	})
	for _, signal := range []string{"notify::month", "notify::year"} {
		connectNativeNotify(unsafe.Pointer(state.widget), signal, state.mark)
	}
	return unsafe.Pointer(state.widget)
}
func gkGTKCalendarDates(pointer unsafe.Pointer, dates *cDouble, capacity cInt) cInt {
	state := componentState[nativeCalendar](pointer)
	if state == nil {
		return 0
	}
	target := unsafe.Slice(dates, int(capacity))
	for i := 0; i < len(target) && i < len(state.dates); i++ {
		target[i] = cDouble(state.dates[i])
	}
	return cInt(len(state.dates))
}
func gkGTKCalendarSetDates(pointer unsafe.Pointer, dates *cDouble, count cInt) {
	state := componentState[nativeCalendar](pointer)
	if state == nil {
		return
	}
	state.dates = make([]float64, int(count))
	for i, date := range unsafe.Slice(dates, int(count)) {
		state.dates[i] = float64(date)
	}
	slices.Sort(state.dates)
	state.anchor = true
	if len(state.dates) > 0 {
		state.showMonth(time.Unix(int64(state.dates[0]), 0).UTC())
	} else {
		state.mark()
	}
}
func gkGTKCalendarVisibleMonth(pointer unsafe.Pointer) cDouble {
	state := componentState[nativeCalendar](pointer)
	if state == nil {
		return 0
	}
	date := nativeCalendarDate(state.widget)
	return cDouble(time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).Unix())
}
func gkGTKCalendarSetVisibleMonth(pointer unsafe.Pointer, month cDouble) {
	state := componentState[nativeCalendar](pointer)
	if state == nil {
		return
	}
	target := time.Unix(int64(month), 0).UTC()
	day := 1
	for _, value := range state.dates {
		date := time.Unix(int64(value), 0).UTC()
		if date.Year() == target.Year() && date.Month() == target.Month() {
			day = date.Day()
		}
	}
	state.showMonth(time.Date(target.Year(), target.Month(), day, 0, 0, 0, 0, time.UTC))
}
