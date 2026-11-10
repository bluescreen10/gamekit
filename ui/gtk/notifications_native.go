//go:build linux && cgo

package gtk

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bluescreen10/gamekit/ui"
)

const notificationBusName = "org.freedesktop.Notifications"
const notificationObjectPath = "/org/freedesktop/Notifications"

type deliveredNotification struct {
	server      uint32
	replyAction string
}

type pendingNotification struct {
	notification ui.Notification
	attachment   string
	timer        cUint
}

var nativeNotifications = struct {
	sync.Mutex
	once            sync.Once
	delivered       map[string]deliveredNotification
	pending         map[string]*pendingNotification
	replyCapability int
}{
	delivered:       make(map[string]deliveredNotification),
	pending:         make(map[string]*pendingNotification),
	replyCapability: -1,
}

func attachNotifications() {
	nativeNotifications.once.Do(func() {
		bus := sessionBus()
		if bus == nil {
			return
		}
		for _, signal := range []string{"ActionInvoked", "NotificationClosed", "NotificationReplied"} {
			gDBusConnectionSignalSubscribe(bus, stringPointer(notificationBusName), stringPointer(notificationBusName),
				stringPointer(signal), stringPointer(notificationObjectPath), nil, 0, gtkDBusSignalCallback(),
				newNativeCallback(func(signal string, parameters *gVariant) { handleNotificationSignal(signal, parameters) }), gtkDestroyCallback())
		}
	})
}

func notificationsAvailable() bool {
	bus := sessionBus()
	if bus == nil {
		return false
	}
	reply, err := callNativeBus(bus, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "NameHasOwner",
		variantTuple(variantString(notificationBusName)), "(b)", 2000)
	if err != nil {
		return false
	}
	defer gVariantUnref(reply)
	owned := gVariantGetChildValue(reply, 0)
	defer gVariantUnref(owned)
	return gVariantGetBoolean(owned) != 0
}

func notificationSupportsReply() bool {
	nativeNotifications.Lock()
	cached := nativeNotifications.replyCapability
	nativeNotifications.Unlock()
	if cached >= 0 {
		return cached != 0
	}
	supported := 0
	reply, err := callNativeBus(sessionBus(), notificationBusName, notificationObjectPath, notificationBusName, "GetCapabilities", nil, "(as)", 2000)
	if err == nil {
		capabilities := gVariantGetChildValue(reply, 0)
		for i := gSize(0); i < gVariantNChildren(capabilities); i++ {
			if variantChildString(capabilities, int(i)) == "inline-reply" {
				supported = 1
				break
			}
		}
		gVariantUnref(capabilities)
		gVariantUnref(reply)
	}
	nativeNotifications.Lock()
	nativeNotifications.replyCapability = supported
	nativeNotifications.Unlock()
	return supported != 0
}

func handleNotificationSignal(signal string, parameters *gVariant) {
	server := variantChildUint32(parameters, 0)
	var identifier string
	var delivered deliveredNotification
	nativeNotifications.Lock()
	for id, notification := range nativeNotifications.delivered {
		if notification.server == server {
			identifier, delivered = id, notification
			if signal == "NotificationClosed" {
				delete(nativeNotifications.delivered, id)
			}
			break
		}
	}
	nativeNotifications.Unlock()
	if identifier == "" || signal == "NotificationClosed" {
		return
	}
	activation := ui.NotificationActivation{NotificationID: identifier}
	value := variantChildString(parameters, 1)
	if signal == "NotificationReplied" {
		activation.ActionID = delivered.replyAction
		activation.ReplyText = value
	} else if value != "default" {
		activation.ActionID = value
	}
	for _, window := range liveWindows() {
		each(&window.events.notifications, func(handler ui.NotificationActivationHandler) { handler(activation) })
	}
}

func deliverNativeNotification(notification ui.Notification, attachment string) error {
	bus := sessionBus()
	if bus == nil {
		return errors.New("ui/gtk: there is no session bus")
	}
	actions := []string{"default", ""}
	replyAction := ""
	for _, action := range notification.Actions {
		if action.TextInput && replyAction == "" && notificationSupportsReply() {
			replyAction = action.ID
			actions = append(actions, "inline-reply", action.Text)
		} else {
			actions = append(actions, action.ID, action.Text)
		}
	}
	category := notification.Category
	if category == "" && len(notification.Actions) > 0 {
		category = "gamekit." + notification.ID
	}
	hints := make(map[string]*gVariant)
	if category != "" {
		hints["category"] = variantString(category)
	}
	if notification.Sound {
		hints["sound-name"] = variantString("message-new-instant")
	} else {
		hints["suppress-sound"] = gVariantNewBoolean(1)
	}
	if attachment != "" {
		hints["image-path"] = variantString(attachment)
	}
	if name := goString(gGetPrgname()); name != "" {
		hints["desktop-entry"] = variantString(name)
	}
	application := goString(gGetApplicationName())
	if application == "" {
		application = "GameKit"
	}
	body := notification.Body
	if notification.Subtitle != "" {
		body = notification.Subtitle + "\n" + body
	}
	nativeNotifications.Lock()
	replaces := nativeNotifications.delivered[notification.ID].server
	nativeNotifications.Unlock()
	reply, err := callNativeBus(bus, notificationBusName, notificationObjectPath, notificationBusName, "Notify",
		variantTuple(variantString(application), gVariantNewUint32(cUint32(replaces)), variantString(""), variantString(notification.Title),
			variantString(body), variantStrings(actions), variantDictionary(hints), gVariantNewInt32(-1)), "(u)", 5000)
	if err != nil {
		return err
	}
	server := variantChildUint32(reply, 0)
	gVariantUnref(reply)
	nativeNotifications.Lock()
	nativeNotifications.delivered[notification.ID] = deliveredNotification{server: server, replyAction: replyAction}
	nativeNotifications.Unlock()
	return nil
}

func sendNativeNotification(notification ui.Notification, attachment string) error {
	cancelPendingNotification(notification.ID)
	delay := time.Until(notification.Delivery)
	if notification.Delivery.IsZero() || delay <= 0 {
		return deliverNativeNotification(notification, attachment)
	}
	// Scheduled work owns a snapshot of the actions, not the caller's slice.
	notification.Actions = slices.Clone(notification.Actions)
	pending := &pendingNotification{notification: notification, attachment: attachment}
	nativeNotifications.Lock()
	pending.timer = gTimeoutAddFull(0, cUint(max(1, delay.Milliseconds())), gtkSourceCallback(), newNativeCallback(func() bool {
		nativeNotifications.Lock()
		pending.timer = 0
		delete(nativeNotifications.pending, notification.ID)
		nativeNotifications.Unlock()
		deliverNativeNotification(pending.notification, pending.attachment)
		return false
	}), gtkDestroyCallback())
	nativeNotifications.pending[notification.ID] = pending
	nativeNotifications.Unlock()
	return nil
}

func cancelPendingNotification(identifier string) {
	nativeNotifications.Lock()
	pending := nativeNotifications.pending[identifier]
	delete(nativeNotifications.pending, identifier)
	if pending != nil && pending.timer != 0 {
		gSourceRemove(pending.timer)
	}
	nativeNotifications.Unlock()
}

func pendingNotificationIDs() []string {
	nativeNotifications.Lock()
	defer nativeNotifications.Unlock()
	return slices.Collect(maps.Keys(nativeNotifications.pending))
}

func deliveredNotificationIDs() []string {
	nativeNotifications.Lock()
	defer nativeNotifications.Unlock()
	return slices.Collect(maps.Keys(nativeNotifications.delivered))
}

func withdrawNativeNotifications(ids []string) {
	for _, id := range ids {
		cancelPendingNotification(id)
		nativeNotifications.Lock()
		server := nativeNotifications.delivered[id].server
		delete(nativeNotifications.delivered, id)
		nativeNotifications.Unlock()
		if server != 0 {
			reply, _ := callNativeBus(sessionBus(), notificationBusName, notificationObjectPath, notificationBusName, "CloseNotification",
				variantTuple(gVariantNewUint32(cUint32(server))), "", 2000)
			if reply != nil {
				gVariantUnref(reply)
			}
		}
	}
}

func setNativeBadge(badge string) {
	bus := sessionBus()
	if bus == nil {
		return
	}
	count, err := strconv.ParseInt(badge, 10, 64)
	if err != nil {
		count = 0
	}
	name := goString(gGetPrgname())
	if name == "" {
		name = "gamekit"
	}
	visible := gBoolean(0)
	if badge != "" {
		visible = 1
	}
	parameters := variantTuple(variantString("application://"+name+".desktop"), variantDictionary(map[string]*gVariant{
		"count": gVariantNewInt64(cInt64(count)), "count-visible": gVariantNewBoolean(visible),
	}))
	gDBusConnectionEmitSignal(bus, nil, stringPointer("/com/canonical/unity/launcherentry/gamekit"),
		stringPointer("com.canonical.Unity.LauncherEntry"), stringPointer("Update"), parameters, nil)
	gDBusConnectionFlushSync(bus, nil, nil)
}
