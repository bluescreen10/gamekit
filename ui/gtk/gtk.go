//go:build linux && cgo

package gtk

/*
#cgo pkg-config: gtk4 gmodule-2.0 x11
#define GDK_DISABLE_DEPRECATION_WARNINGS
#define GTK_DISABLE_DEPRECATION_WARNINGS
#include <gtk/gtk.h>
#include <gmodule.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <gdk/x11/gdkx.h>
#include <pango/pangocairo.h>

extern void goGTKLayoutClass(void *, void *);
extern void goGTKImageClass(void *, void *);
extern void goGTKLayoutMeasure(GtkWidget *, GtkOrientation, int, int *, int *, int *, int *);
extern void goGTKImageMeasure(GtkWidget *, GtkOrientation, int, int *, int *, int *, int *);
extern void goGTKLayoutAllocate(GtkWidget *, int, int, int);
extern void goGTKLayoutSnapshot(GtkWidget *, GtkSnapshot *);
extern void goGTKImageSnapshot(GtkWidget *, GtkSnapshot *);
extern gboolean goGTKRangeValue(void *instance, GtkScrollType scroll, double value, void *data);
extern void goGTKTabSwitch(void *instance, void *page, unsigned int position, void *data);
extern gboolean goGTKOverlayPosition(void *instance, GtkWidget *child, GdkRectangle *allocation, void *data);
extern void goGTKTextInsert(void *instance, GtkTextIter *location, char *text, int length, void *data);
extern GdkContentProvider * goGTKDragPrepare(void *instance, double x, double y, void *data);
extern gboolean goGTKDropAccept(void *instance, GdkDrop *drop, void *data);
extern GdkDragAction goGTKDropMotion(void *instance, GdkDrop *drop, double x, double y, void *data);
extern gboolean goGTKDropPerform(void *instance, GdkDrop *drop, double x, double y, void *data);
extern gboolean goGTKLegacyEvent(void *instance, GdkEvent *event, void *data);
extern void goGTKStringSignal(void *instance, char *text, void *data);
extern GdkDragAction goGTKDropEnter(void *instance, double x, double y, void *data);
extern gboolean goGTKValueDrop(void *instance, GValue *value, double x, double y, void *data);
extern gboolean goGTKScrollSignal(void *instance, double x, double y, void *data);
extern void goGTKKeyReleased(void *instance, unsigned int keyval, unsigned int keycode, GdkModifierType state, void *data);
extern void goGTKPrintBegin(GtkPrintOperation *, GtkPrintContext *, void *);
extern void goGTKPrintDraw(GtkPrintOperation *, GtkPrintContext *, int, void *);
extern void goGTKSignal(void *, void *);
extern void goGTKCallbackFree(void *, GClosure *);
extern void goGTKAsyncReady(GObject *, GAsyncResult *, void *);
extern gboolean goGTKCloseRequest(void *, void *);
extern gboolean goGTKSource(void *);
extern void goGTKDestroy(void *);
extern void goGTKDBusSignal(GDBusConnection *, char *, char *, char *, char *, GVariant *, void *);
extern void goGTKAction(void *, GVariant *, void *);
extern void goGTKObjectSignal(void *, void *, void *);
extern void goGTKXYSignal(void *, double, double, void *);
extern gboolean goGTKKeySignal(void *, unsigned int, unsigned int, GdkModifierType, void *);
extern void goGTKDraw(GtkDrawingArea *, cairo_t *, int, int, void *);
extern void goGTKListItem(void *, GtkListItem *, void *);
extern void goGTKUIntSignal(void *, unsigned int, void *);
extern void goGTKSelectionSignal(void *, unsigned int, unsigned int, void *);
extern void goGTKIntSignal(void *, int, void *);
extern GListModel *goGTKTreeChildren(void *, void *);
extern int goGTKEqualSort(void *, void *, void *);
extern gboolean goGTKTick(GtkWidget *, GdkFrameClock *, void *);
extern void goGTKNotify(void *, GParamSpec *, void *);
extern void goGTKPressed(void *, int, double, double, void *);
extern void goGTKDBusMethod(GDBusConnection *, char *, char *, char *, char *, GVariant *, GDBusMethodInvocation *, void *);
extern GVariant *goGTKDBusProperty(GDBusConnection *, char *, char *, char *, char *, GError **, void *);
extern void goGTKBusAppeared(GDBusConnection *, char *, char *, void *);
extern void goGTKBusVanished(GDBusConnection *, char *, void *);

// Indirect calls for optional libraries. Symbol loading and all decisions live in Go.
typedef struct _WebKitWebView WebKitWebView;
typedef struct _WebKitSettings WebKitSettings;
static WebKitWebView *gkCallWebNew(void *function) { return ((WebKitWebView *(*)(void))function)(); }
static void gkCallWebURI(void *function, WebKitWebView *view, char *uri) { ((void (*)(WebKitWebView *, const char *))function)(view, uri); }
static void gkCallWebHTML(void *function, WebKitWebView *view, char *html, char *base) { ((void (*)(WebKitWebView *, const char *, const char *))function)(view, html, base); }
static const char *gkCallWebGetURI(void *function, WebKitWebView *view) { return ((const char *(*)(WebKitWebView *))function)(view); }
static WebKitSettings *gkCallWebSettings(void *function, WebKitWebView *view) { return ((WebKitSettings *(*)(WebKitWebView *))function)(view); }
static void gkCallWebJavaScript(void *function, WebKitSettings *settings, gboolean enabled) { ((void (*)(WebKitSettings *, gboolean))function)(settings, enabled); }
static void gkCallWebStop(void *function, WebKitWebView *view) { ((void (*)(WebKitWebView *))function)(view); }
static GdkPaintable *gkCallSVGNew(void *function, GBytes *bytes) { return ((GdkPaintable *(*)(GBytes *))function)(bytes); }
*/
import "C"

import "unsafe"

// Native ABI aliases keep these wrappers replaceable by //cgo:binding declarations.
type (
	atom                              = C.Atom
	cChar                             = C.char
	cDouble                           = C.double
	cFloat                            = C.float
	cInt                              = C.int
	cInt64                            = C.gint64
	cLong                             = C.long
	cSize                             = C.size_t
	cUchar                            = C.uchar
	cUint                             = C.guint
	cUint32                           = C.guint32
	cUintptr                          = C.uintptr_t
	cUlong                            = C.ulong
	cairoContext                      = C.cairo_t
	cairoFormat                       = C.cairo_format_t
	cairoSurface                      = C.cairo_surface_t
	cursor                            = C.Cursor
	display                           = C.Display
	gAction                           = C.GAction
	gActionGroup                      = C.GActionGroup
	gActionMap                        = C.GActionMap
	gAppInfo                          = C.GAppInfo
	gAppLaunchContext                 = C.GAppLaunchContext
	gAsyncReadyCallback               = C.GAsyncReadyCallback
	gAsyncResult                      = C.GAsyncResult
	gBoolean                          = C.gboolean
	gBusNameAppearedCallback          = C.GBusNameAppearedCallback
	gBusNameVanishedCallback          = C.GBusNameVanishedCallback
	gBusNameWatcherFlags              = C.GBusNameWatcherFlags
	gBusType                          = C.GBusType
	gBytes                            = C.GBytes
	gCallback                         = C.GCallback
	gCancellable                      = C.GCancellable
	gClassInitFunc                    = C.GClassInitFunc
	gClosure                          = C.GClosure
	gClosureNotify                    = C.GClosureNotify
	gCompareDataFunc                  = C.GCompareDataFunc
	gConnectFlags                     = C.GConnectFlags
	gDBusAuthObserver                 = C.GDBusAuthObserver
	gDBusCallFlags                    = C.GDBusCallFlags
	gDBusConnection                   = C.GDBusConnection
	gDBusConnectionFlags              = C.GDBusConnectionFlags
	gDBusInterfaceGetPropertyFunc     = C.GDBusInterfaceGetPropertyFunc
	gDBusInterfaceInfo                = C.GDBusInterfaceInfo
	gDBusInterfaceMethodCallFunc      = C.GDBusInterfaceMethodCallFunc
	gDBusInterfaceVTable              = C.GDBusInterfaceVTable
	gDBusMethodInvocation             = C.GDBusMethodInvocation
	gDBusNodeInfo                     = C.GDBusNodeInfo
	gDBusSignalCallback               = C.GDBusSignalCallback
	gDBusSignalFlags                  = C.GDBusSignalFlags
	gDateTime                         = C.GDateTime
	gDestroyNotify                    = C.GDestroyNotify
	gEnumClass                        = C.GEnumClass
	gEnumValue                        = C.GEnumValue
	gError                            = C.GError
	gFile                             = C.GFile
	gFileQueryInfoFlags               = C.GFileQueryInfoFlags
	gFileType                         = C.GFileType
	gIcon                             = C.GIcon
	gInputStream                      = C.GInputStream
	gInstanceInitFunc                 = C.GInstanceInitFunc
	gList                             = C.GList
	gListModel                        = C.GListModel
	gListStore                        = C.GListStore
	gMainContext                      = C.GMainContext
	gMenu                             = C.GMenu
	gMenuItem                         = C.GMenuItem
	gMenuModel                        = C.GMenuModel
	gModule                           = C.GModule
	gModuleFlags                      = C.GModuleFlags
	gObject                           = C.GObject
	gObjectClass                      = C.GObjectClass
	gParamSpec                        = C.GParamSpec
	gQuark                            = C.GQuark
	gSList                            = C.GSList
	gSettings                         = C.GSettings
	gSettingsSchema                   = C.GSettingsSchema
	gSettingsSchemaSource             = C.GSettingsSchemaSource
	gSimpleAction                     = C.GSimpleAction
	gSimpleActionGroup                = C.GSimpleActionGroup
	gSize                             = C.gsize
	gSourceFunc                       = C.GSourceFunc
	gType                             = C.GType
	gTypeFlags                        = C.GTypeFlags
	gTypeInstance                     = C.GTypeInstance
	gUlong                            = C.gulong
	gValue                            = C.GValue
	gVariant                          = C.GVariant
	gVariantType                      = C.GVariantType
	gchar                             = C.gchar
	gdkAxisUse                        = C.GdkAxisUse
	gdkClipboard                      = C.GdkClipboard
	gdkContentFormats                 = C.GdkContentFormats
	gdkContentFormatsBuilder          = C.GdkContentFormatsBuilder
	gdkContentProvider                = C.GdkContentProvider
	gdkDevice                         = C.GdkDevice
	gdkDeviceTool                     = C.GdkDeviceTool
	gdkDeviceToolType                 = C.GdkDeviceToolType
	gdkDisplay                        = C.GdkDisplay
	gdkDragAction                     = C.GdkDragAction
	gdkDrop                           = C.GdkDrop
	gdkEvent                          = C.GdkEvent
	gdkEventSequence                  = C.GdkEventSequence
	gdkEventType                      = C.GdkEventType
	gdkFileList                       = C.GdkFileList
	gdkFrameClock                     = C.GdkFrameClock
	gdkInputSource                    = C.GdkInputSource
	gdkMemoryFormat                   = C.GdkMemoryFormat
	gdkModifierType                   = C.GdkModifierType
	gdkMonitor                        = C.GdkMonitor
	gdkNotifyType                     = C.GdkNotifyType
	gdkPaintable                      = C.GdkPaintable
	gdkRGBA                           = C.GdkRGBA
	gdkRectangle                      = C.GdkRectangle
	gdkScrollDirection                = C.GdkScrollDirection
	gdkSeat                           = C.GdkSeat
	gdkSurface                        = C.GdkSurface
	gdkTexture                        = C.GdkTexture
	gdkTextureDownloader              = C.GdkTextureDownloader
	gdkToplevel                       = C.GdkToplevel
	gdkToplevelState                  = C.GdkToplevelState
	gdkTouchpadGesturePhase           = C.GdkTouchpadGesturePhase
	gdouble                           = C.gdouble
	gint                              = C.gint
	gint32                            = C.gint32
	gpointer                          = C.gpointer
	graphenePoint                     = C.graphene_point_t
	grapheneRect                      = C.graphene_rect_t
	grapheneSize                      = C.graphene_size_t
	gskScalingFilter                  = C.GskScalingFilter
	gskTransform                      = C.GskTransform
	gtkAboutDialog                    = C.GtkAboutDialog
	gtkAccessible                     = C.GtkAccessible
	gtkAccessibleAnnouncementPriority = C.GtkAccessibleAnnouncementPriority
	gtkAccessibleProperty             = C.GtkAccessibleProperty
	gtkAccessibleRelation             = C.GtkAccessibleRelation
	gtkAccessibleRole                 = C.GtkAccessibleRole
	gtkAccessibleState                = C.GtkAccessibleState
	gtkAdjustment                     = C.GtkAdjustment
	gtkAlign                          = C.GtkAlign
	gtkAllocation                     = C.GtkAllocation
	gtkBitset                         = C.GtkBitset
	gtkBox                            = C.GtkBox
	gtkButton                         = C.GtkButton
	gtkCalendar                       = C.GtkCalendar
	gtkCenterBox                      = C.GtkCenterBox
	gtkCheckButton                    = C.GtkCheckButton
	gtkColorDialog                    = C.GtkColorDialog
	gtkColorDialogButton              = C.GtkColorDialogButton
	gtkColumnView                     = C.GtkColumnView
	gtkColumnViewColumn               = C.GtkColumnViewColumn
	gtkColumnViewSorter               = C.GtkColumnViewSorter
	gtkContentFit                     = C.GtkContentFit
	gtkCssProvider                    = C.GtkCssProvider
	gtkCustomSorter                   = C.GtkCustomSorter
	gtkDragSource                     = C.GtkDragSource
	gtkDrawingArea                    = C.GtkDrawingArea
	gtkDrawingAreaDrawFunc            = C.GtkDrawingAreaDrawFunc
	gtkDropDown                       = C.GtkDropDown
	gtkDropTarget                     = C.GtkDropTarget
	gtkDropTargetAsync                = C.GtkDropTargetAsync
	gtkEditable                       = C.GtkEditable
	gtkEntry                          = C.GtkEntry
	gtkEventController                = C.GtkEventController
	gtkEventControllerScrollFlags     = C.GtkEventControllerScrollFlags
	gtkEventSequenceState             = C.GtkEventSequenceState
	gtkExpander                       = C.GtkExpander
	gtkExpression                     = C.GtkExpression
	gtkFileDialog                     = C.GtkFileDialog
	gtkFileFilter                     = C.GtkFileFilter
	gtkFileLauncher                   = C.GtkFileLauncher
	gtkFlattenListModel               = C.GtkFlattenListModel
	gtkFlowBoxChild                   = C.GtkFlowBoxChild
	gtkFontDialog                     = C.GtkFontDialog
	gtkFontDialogButton               = C.GtkFontDialogButton
	gtkFontLevel                      = C.GtkFontLevel
	gtkFrame                          = C.GtkFrame
	gtkGesture                        = C.GtkGesture
	gtkGestureSingle                  = C.GtkGestureSingle
	gtkGrid                           = C.GtkGrid
	gtkGridView                       = C.GtkGridView
	gtkIMContext                      = C.GtkIMContext
	gtkIconLookupFlags                = C.GtkIconLookupFlags
	gtkIconPaintable                  = C.GtkIconPaintable
	gtkIconTheme                      = C.GtkIconTheme
	gtkImage                          = C.GtkImage
	gtkInputHints                     = C.GtkInputHints
	gtkInputPurpose                   = C.GtkInputPurpose
	gtkJustification                  = C.GtkJustification
	gtkLabel                          = C.GtkLabel
	gtkLayoutManager                  = C.GtkLayoutManager
	gtkLevelBar                       = C.GtkLevelBar
	gtkLinkButton                     = C.GtkLinkButton
	gtkListBox                        = C.GtkListBox
	gtkListBoxRow                     = C.GtkListBoxRow
	gtkListItem                       = C.GtkListItem
	gtkListItemFactory                = C.GtkListItemFactory
	gtkListScrollFlags                = C.GtkListScrollFlags
	gtkListView                       = C.GtkListView
	gtkMediaStream                    = C.GtkMediaStream
	gtkMenuButton                     = C.GtkMenuButton
	gtkMultiSelection                 = C.GtkMultiSelection
	gtkNative                         = C.GtkNative
	gtkNoSelection                    = C.GtkNoSelection
	gtkNotebook                       = C.GtkNotebook
	gtkOrientable                     = C.GtkOrientable
	gtkOrientation                    = C.GtkOrientation
	gtkOverflow                       = C.GtkOverflow
	gtkOverlay                        = C.GtkOverlay
	gtkPaned                          = C.GtkPaned
	gtkPasswordEntry                  = C.GtkPasswordEntry
	gtkPicture                        = C.GtkPicture
	gtkPolicyType                     = C.GtkPolicyType
	gtkPopover                        = C.GtkPopover
	gtkPositionType                   = C.GtkPositionType
	gtkPrintContext                   = C.GtkPrintContext
	gtkPrintOperation                 = C.GtkPrintOperation
	gtkPrintOperationAction           = C.GtkPrintOperationAction
	gtkPrintOperationResult           = C.GtkPrintOperationResult
	gtkProgressBar                    = C.GtkProgressBar
	gtkPropagationPhase               = C.GtkPropagationPhase
	gtkRange                          = C.GtkRange
	gtkRecentInfo                     = C.GtkRecentInfo
	gtkRecentManager                  = C.GtkRecentManager
	gtkRevealer                       = C.GtkRevealer
	gtkRevealerTransitionType         = C.GtkRevealerTransitionType
	gtkRoot                           = C.GtkRoot
	gtkScale                          = C.GtkScale
	gtkScrollInfo                     = C.GtkScrollInfo
	gtkScrolledWindow                 = C.GtkScrolledWindow
	gtkSearchEntry                    = C.GtkSearchEntry
	gtkSelectionModel                 = C.GtkSelectionModel
	gtkSettings                       = C.GtkSettings
	gtkShortcut                       = C.GtkShortcut
	gtkShortcutAction                 = C.GtkShortcutAction
	gtkShortcutController             = C.GtkShortcutController
	gtkShortcutScope                  = C.GtkShortcutScope
	gtkShortcutTrigger                = C.GtkShortcutTrigger
	gtkSingleSelection                = C.GtkSingleSelection
	gtkSnapshot                       = C.GtkSnapshot
	gtkSortType                       = C.GtkSortType
	gtkSorter                         = C.GtkSorter
	gtkSpinButton                     = C.GtkSpinButton
	gtkSpinner                        = C.GtkSpinner
	gtkStack                          = C.GtkStack
	gtkStringList                     = C.GtkStringList
	gtkStringObject                   = C.GtkStringObject
	gtkStyleProvider                  = C.GtkStyleProvider
	gtkSwitch                         = C.GtkSwitch
	gtkText                           = C.GtkText
	gtkTextBuffer                     = C.GtkTextBuffer
	gtkTextDirection                  = C.GtkTextDirection
	gtkTextIter                       = C.GtkTextIter
	gtkTextTag                        = C.GtkTextTag
	gtkTextTagTable                   = C.GtkTextTagTable
	gtkTextView                       = C.GtkTextView
	gtkTextWindowType                 = C.GtkTextWindowType
	gtkTickCallback                   = C.GtkTickCallback
	gtkToggleButton                   = C.GtkToggleButton
	gtkTreeExpander                   = C.GtkTreeExpander
	gtkTreeListModel                  = C.GtkTreeListModel
	gtkTreeListModelCreateModelFunc   = C.GtkTreeListModelCreateModelFunc
	gtkTreeListRow                    = C.GtkTreeListRow
	gtkUnit                           = C.GtkUnit
	gtkUriLauncher                    = C.GtkUriLauncher
	gtkVideo                          = C.GtkVideo
	gtkViewport                       = C.GtkViewport
	gtkWidget                         = C.GtkWidget
	gtkWidgetClass                    = C.GtkWidgetClass
	gtkWindow                         = C.GtkWindow
	gtkWrapMode                       = C.GtkWrapMode
	pangoAttrList                     = C.PangoAttrList
	pangoEllipsizeMode                = C.PangoEllipsizeMode
	pangoFontDescription              = C.PangoFontDescription
	pangoFontFamily                   = C.PangoFontFamily
	pangoFontMap                      = C.PangoFontMap
	pangoLayout                       = C.PangoLayout
	pangoStyle                        = C.PangoStyle
	pangoWeight                       = C.PangoWeight
	pangoWrapMode                     = C.PangoWrapMode
	webkitSettings                    = C.WebKitSettings
	webkitWebView                     = C.WebKitWebView
	xClientMessageEvent               = C.XClientMessageEvent
	xEvent                            = C.XEvent
	xSizeHints                        = C.XSizeHints
	xTime                             = C.Time
	xWindow                           = C.Window
	xWindowAttributes                 = C.XWindowAttributes
)

const (
	cairoFormatARGB32                        = C.CAIRO_FORMAT_ARGB32
	gBusTypeSession                          = C.G_BUS_TYPE_SESSION
	gConnectAfter                            = C.G_CONNECT_AFTER
	gDBusCallFlagsNone                       = C.G_DBUS_CALL_FLAGS_NONE
	gDBusConnectionFlagsAuthenticationClient = C.G_DBUS_CONNECTION_FLAGS_AUTHENTICATION_CLIENT
	gDBusConnectionFlagsMessageBusConnection = C.G_DBUS_CONNECTION_FLAGS_MESSAGE_BUS_CONNECTION
	gFileQueryInfoNone                       = C.G_FILE_QUERY_INFO_NONE
	gFileTypeDirectory                       = C.G_FILE_TYPE_DIRECTORY
	gTypeBoolean                             = C.G_TYPE_BOOLEAN
	gTypeDouble                              = C.G_TYPE_DOUBLE
	gTypeEnum                                = C.G_TYPE_ENUM
	gTypeFlagsKind                           = C.G_TYPE_FLAGS
	gTypeInt                                 = C.G_TYPE_INT
	gTypeString                              = C.G_TYPE_STRING
	gTypeUint                                = C.G_TYPE_UINT
	gdkActionCopy                            = C.GDK_ACTION_COPY
	gdkActionLink                            = C.GDK_ACTION_LINK
	gdkActionMove                            = C.GDK_ACTION_MOVE
	gdkAltMask                               = C.GDK_ALT_MASK
	gdkAxisPressure                          = C.GDK_AXIS_PRESSURE
	gdkAxisRotation                          = C.GDK_AXIS_ROTATION
	gdkAxisXtilt                             = C.GDK_AXIS_XTILT
	gdkAxisYtilt                             = C.GDK_AXIS_YTILT
	gdkButton2Mask                           = C.GDK_BUTTON2_MASK
	gdkButtonPress                           = C.GDK_BUTTON_PRESS
	gdkButtonRelease                         = C.GDK_BUTTON_RELEASE
	gdkControlMask                           = C.GDK_CONTROL_MASK
	gdkDeviceToolTypeEraser                  = C.GDK_DEVICE_TOOL_TYPE_ERASER
	gdkEnterNotify                           = C.GDK_ENTER_NOTIFY
	gdkKeyAltL                               = C.GDK_KEY_Alt_L
	gdkKeyAltR                               = C.GDK_KEY_Alt_R
	gdkKeyBackSpace                          = C.GDK_KEY_BackSpace
	gdkKeyControlL                           = C.GDK_KEY_Control_L
	gdkKeyControlR                           = C.GDK_KEY_Control_R
	gdkKeyDelete                             = C.GDK_KEY_Delete
	gdkKeyDown                               = C.GDK_KEY_Down
	gdkKeyEscape                             = C.GDK_KEY_Escape
	gdkKeyLeft                               = C.GDK_KEY_Left
	gdkKeyMetaL                              = C.GDK_KEY_Meta_L
	gdkKeyMetaR                              = C.GDK_KEY_Meta_R
	gdkKeyPress                              = C.GDK_KEY_PRESS
	gdkKeyRelease                            = C.GDK_KEY_RELEASE
	gdkKeyRight                              = C.GDK_KEY_Right
	gdkKeyShiftL                             = C.GDK_KEY_Shift_L
	gdkKeyShiftR                             = C.GDK_KEY_Shift_R
	gdkKeySuperL                             = C.GDK_KEY_Super_L
	gdkKeySuperR                             = C.GDK_KEY_Super_R
	gdkKeyTab                                = C.GDK_KEY_Tab
	gdkKeyUp                                 = C.GDK_KEY_Up
	gdkLeaveNotify                           = C.GDK_LEAVE_NOTIFY
	gdkLockMask                              = C.GDK_LOCK_MASK
	gdkMemoryR8G8B8A8                        = C.GDK_MEMORY_R8G8B8A8
	gdkMetaMask                              = C.GDK_META_MASK
	gdkMotionNotify                          = C.GDK_MOTION_NOTIFY
	gdkNotifyInferior                        = C.GDK_NOTIFY_INFERIOR
	gdkScroll                                = C.GDK_SCROLL
	gdkScrollDown                            = C.GDK_SCROLL_DOWN
	gdkScrollLeft                            = C.GDK_SCROLL_LEFT
	gdkScrollRight                           = C.GDK_SCROLL_RIGHT
	gdkScrollUp                              = C.GDK_SCROLL_UP
	gdkShiftMask                             = C.GDK_SHIFT_MASK
	gdkSourceKeyboard                        = C.GDK_SOURCE_KEYBOARD
	gdkSourcePen                             = C.GDK_SOURCE_PEN
	gdkSourceTouchscreen                     = C.GDK_SOURCE_TOUCHSCREEN
	gdkSuperMask                             = C.GDK_SUPER_MASK
	gdkToplevelStateMinimized                = C.GDK_TOPLEVEL_STATE_MINIMIZED
	gdkTouchBegin                            = C.GDK_TOUCH_BEGIN
	gdkTouchCancel                           = C.GDK_TOUCH_CANCEL
	gdkTouchEnd                              = C.GDK_TOUCH_END
	gdkTouchUpdate                           = C.GDK_TOUCH_UPDATE
	gdkTouchpadGesturePhaseBegin             = C.GDK_TOUCHPAD_GESTURE_PHASE_BEGIN
	gdkTouchpadGesturePhaseCancel            = C.GDK_TOUCHPAD_GESTURE_PHASE_CANCEL
	gdkTouchpadGesturePhaseEnd               = C.GDK_TOUCHPAD_GESTURE_PHASE_END
	gdkTouchpadPinch                         = C.GDK_TOUCHPAD_PINCH
	gdkTouchpadSwipe                         = C.GDK_TOUCHPAD_SWIPE
	gskScalingFilterNearest                  = gskScalingFilter(C.GSK_SCALING_FILTER_NEAREST)
	gskScalingFilterTrilinear                = gskScalingFilter(C.GSK_SCALING_FILTER_TRILINEAR)
	gtkAccessibleAnnouncementPriorityHigh    = gtkAccessibleAnnouncementPriority(C.GTK_ACCESSIBLE_ANNOUNCEMENT_PRIORITY_HIGH)
	gtkAccessibleAnnouncementPriorityMedium  = gtkAccessibleAnnouncementPriority(C.GTK_ACCESSIBLE_ANNOUNCEMENT_PRIORITY_MEDIUM)
	gtkAccessibleInvalidFalse                = C.GTK_ACCESSIBLE_INVALID_FALSE
	gtkAccessibleInvalidTrue                 = C.GTK_ACCESSIBLE_INVALID_TRUE
	gtkAccessiblePropertyDescription         = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_DESCRIPTION)
	gtkAccessiblePropertyLabel               = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_LABEL)
	gtkAccessiblePropertyPlaceholder         = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_PLACEHOLDER)
	gtkAccessiblePropertyRequired            = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_REQUIRED)
	gtkAccessiblePropertyValueMax            = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_VALUE_MAX)
	gtkAccessiblePropertyValueMin            = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_VALUE_MIN)
	gtkAccessiblePropertyValueNow            = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_VALUE_NOW)
	gtkAccessiblePropertyValueText           = gtkAccessibleProperty(C.GTK_ACCESSIBLE_PROPERTY_VALUE_TEXT)
	gtkAccessibleRelationDescribedBy         = gtkAccessibleRelation(C.GTK_ACCESSIBLE_RELATION_DESCRIBED_BY)
	gtkAccessibleRelationLabelledBy          = gtkAccessibleRelation(C.GTK_ACCESSIBLE_RELATION_LABELLED_BY)
	gtkAccessibleRoleAlert                   = C.GTK_ACCESSIBLE_ROLE_ALERT
	gtkAccessibleRoleAlertDialog             = C.GTK_ACCESSIBLE_ROLE_ALERT_DIALOG
	gtkAccessibleRoleApplication             = C.GTK_ACCESSIBLE_ROLE_APPLICATION
	gtkAccessibleRoleComboBox                = C.GTK_ACCESSIBLE_ROLE_COMBO_BOX
	gtkAccessibleRoleGroup                   = C.GTK_ACCESSIBLE_ROLE_GROUP
	gtkAccessibleRoleImg                     = C.GTK_ACCESSIBLE_ROLE_IMG
	gtkAccessibleRoleNavigation              = C.GTK_ACCESSIBLE_ROLE_NAVIGATION
	gtkAccessibleRoleRadioGroup              = C.GTK_ACCESSIBLE_ROLE_RADIO_GROUP
	gtkAccessibleRoleSeparator               = C.GTK_ACCESSIBLE_ROLE_SEPARATOR
	gtkAccessibleRoleSlider                  = C.GTK_ACCESSIBLE_ROLE_SLIDER
	gtkAccessibleRoleSpinButton              = C.GTK_ACCESSIBLE_ROLE_SPIN_BUTTON
	gtkAccessibleRoleStatus                  = C.GTK_ACCESSIBLE_ROLE_STATUS
	gtkAccessibleRoleToolbar                 = C.GTK_ACCESSIBLE_ROLE_TOOLBAR
	gtkAccessibleRoleTooltip                 = C.GTK_ACCESSIBLE_ROLE_TOOLTIP
	gtkAccessibleStateBusy                   = gtkAccessibleState(C.GTK_ACCESSIBLE_STATE_BUSY)
	gtkAccessibleStateExpanded               = gtkAccessibleState(C.GTK_ACCESSIBLE_STATE_EXPANDED)
	gtkAccessibleStateHidden                 = gtkAccessibleState(C.GTK_ACCESSIBLE_STATE_HIDDEN)
	gtkAccessibleStateInvalid                = gtkAccessibleState(C.GTK_ACCESSIBLE_STATE_INVALID)
	gtkAccessibleStateSelected               = gtkAccessibleState(C.GTK_ACCESSIBLE_STATE_SELECTED)
	gtkAlignCenter                           = gtkAlign(C.GTK_ALIGN_CENTER)
	gtkAlignEnd                              = gtkAlign(C.GTK_ALIGN_END)
	gtkAlignFill                             = gtkAlign(C.GTK_ALIGN_FILL)
	gtkAlignStart                            = gtkAlign(C.GTK_ALIGN_START)
	gtkBoxClassSize                          = C.sizeof_GtkBoxClass
	gtkBoxSize                               = C.sizeof_GtkBox
	gtkContentFitContain                     = C.GTK_CONTENT_FIT_CONTAIN
	gtkEventControllerScrollBothAxes         = C.GTK_EVENT_CONTROLLER_SCROLL_BOTH_AXES
	gtkEventSequenceClaimed                  = C.GTK_EVENT_SEQUENCE_CLAIMED
	gtkFontLevelFont                         = C.GTK_FONT_LEVEL_FONT
	gtkInputHintNoSpellcheck                 = C.GTK_INPUT_HINT_NO_SPELLCHECK
	gtkInputHintSpellcheck                   = C.GTK_INPUT_HINT_SPELLCHECK
	gtkInputPurposeEmail                     = C.GTK_INPUT_PURPOSE_EMAIL
	gtkInputPurposeFreeForm                  = C.GTK_INPUT_PURPOSE_FREE_FORM
	gtkInputPurposeName                      = C.GTK_INPUT_PURPOSE_NAME
	gtkInputPurposePhone                     = C.GTK_INPUT_PURPOSE_PHONE
	gtkInputPurposeURL                       = C.GTK_INPUT_PURPOSE_URL
	gtkJustifyCenter                         = C.GTK_JUSTIFY_CENTER
	gtkOrientationHorizontal                 = gtkOrientation(C.GTK_ORIENTATION_HORIZONTAL)
	gtkOrientationVertical                   = gtkOrientation(C.GTK_ORIENTATION_VERTICAL)
	gtkOverflowVisible                       = C.GTK_OVERFLOW_VISIBLE
	gtkPhaseCapture                          = C.GTK_PHASE_CAPTURE
	gtkPolicyAlways                          = gtkPolicyType(C.GTK_POLICY_ALWAYS)
	gtkPolicyAutomatic                       = gtkPolicyType(C.GTK_POLICY_AUTOMATIC)
	gtkPolicyExternal                        = gtkPolicyType(C.GTK_POLICY_EXTERNAL)
	gtkPolicyNever                           = gtkPolicyType(C.GTK_POLICY_NEVER)
	gtkPosBottom                             = gtkPositionType(C.GTK_POS_BOTTOM)
	gtkPosLeft                               = gtkPositionType(C.GTK_POS_LEFT)
	gtkPosRight                              = gtkPositionType(C.GTK_POS_RIGHT)
	gtkPosTop                                = gtkPositionType(C.GTK_POS_TOP)
	gtkPrintOperationActionExport            = C.GTK_PRINT_OPERATION_ACTION_EXPORT
	gtkPrintOperationActionPrint             = C.GTK_PRINT_OPERATION_ACTION_PRINT
	gtkPrintOperationActionPrintDialog       = C.GTK_PRINT_OPERATION_ACTION_PRINT_DIALOG
	gtkPrintOperationResultApply             = C.GTK_PRINT_OPERATION_RESULT_APPLY
	gtkRevealerTransitionTypeSlideDown       = C.GTK_REVEALER_TRANSITION_TYPE_SLIDE_DOWN
	gtkShortcutScopeGlobal                   = C.GTK_SHORTCUT_SCOPE_GLOBAL
	gtkShortcutScopeManaged                  = C.GTK_SHORTCUT_SCOPE_MANAGED
	gtkSortAscending                         = C.GTK_SORT_ASCENDING
	gtkTextDirNone                           = gtkTextDirection(C.GTK_TEXT_DIR_NONE)
	gtkTextDirRTL                            = gtkTextDirection(C.GTK_TEXT_DIR_RTL)
	gtkTextWindowWidget                      = C.GTK_TEXT_WINDOW_WIDGET
	gtkUnitPoints                            = C.GTK_UNIT_POINTS
	gtkWidgetClassSize                       = C.sizeof_GtkWidgetClass
	gtkWidgetSize                            = C.sizeof_GtkWidget
	gtkWrapChar                              = gtkWrapMode(C.GTK_WRAP_CHAR)
	gtkWrapNone                              = gtkWrapMode(C.GTK_WRAP_NONE)
	gtkWrapWordChar                          = gtkWrapMode(C.GTK_WRAP_WORD_CHAR)
	pangoEllipsizeEnd                        = pangoEllipsizeMode(C.PANGO_ELLIPSIZE_END)
	pangoEllipsizeNone                       = pangoEllipsizeMode(C.PANGO_ELLIPSIZE_NONE)
	pangoScale                               = C.PANGO_SCALE
	pangoStyleItalic                         = C.PANGO_STYLE_ITALIC
	pangoStyleNormal                         = C.PANGO_STYLE_NORMAL
	pangoWeightNormal                        = C.PANGO_WEIGHT_NORMAL
	pangoWrapChar                            = pangoWrapMode(C.PANGO_WRAP_CHAR)
	pangoWrapWordChar                        = pangoWrapMode(C.PANGO_WRAP_WORD_CHAR)
	xAAtom                                   = C.XA_ATOM
	xACardinal                               = C.XA_CARDINAL
	xButtonPressMask                         = C.ButtonPressMask
	xButtonReleaseMask                       = C.ButtonReleaseMask
	xClientMessage                           = C.ClientMessage
	xGrabModeAsync                           = C.GrabModeAsync
	xIconicState                             = C.IconicState
	xIsViewable                              = C.IsViewable
	xPMaxSize                                = C.PMaxSize
	xPMinSize                                = C.PMinSize
	xPointerMotionMask                       = C.PointerMotionMask
	xPropModeReplace                         = C.PropModeReplace
	xSubstructureNotifyMask                  = C.SubstructureNotifyMask
	xSubstructureRedirectMask                = C.SubstructureRedirectMask
	xUSPosition                              = C.USPosition
)

// Native calls. Backend behavior lives in the other Go files.
func xAllocSizeHints() *xSizeHints { return C.XAllocSizeHints() }

func xChangeProperty(display *display, window xWindow, property atom, kind atom, format cInt, mode cInt, data *cUchar, count cInt) cInt {
	return C.XChangeProperty(display, window, property, kind, format, mode, data, count)
}

func xCreateSimpleWindow(display *display, parent xWindow, x cInt, y cInt, width cUint, height cUint, borderWidth cUint, border cUlong, background cUlong) xWindow {
	return C.XCreateSimpleWindow(display, parent, x, y, width, height, borderWidth, border, background)
}

func xDefaultRootWindow(display *display) xWindow { return C.XDefaultRootWindow(display) }

func xDestroyWindow(display *display, window xWindow) cInt { return C.XDestroyWindow(display, window) }

func xFlush(display *display) cInt { return C.XFlush(display) }

func xFree(data unsafe.Pointer) cInt { return C.XFree(data) }

func xGetWMNormalHints(display *display, window xWindow, hints *xSizeHints, supplied *cLong) cInt {
	return C.XGetWMNormalHints(display, window, hints, supplied)
}

func xGetWindowAttributes(display *display, window xWindow, attributes *xWindowAttributes) cInt {
	return C.XGetWindowAttributes(display, window, attributes)
}

func xGetWindowProperty(display *display, window xWindow, property atom, offset cLong, length cLong, deleteProperty cInt, requestedType atom, actualType *atom, format *cInt, count *cUlong, remaining *cUlong, data **cUchar) cInt {
	return C.XGetWindowProperty(display, window, property, offset, length, deleteProperty, requestedType, actualType, format, count, remaining, data)
}

func xGrabPointer(display *display, window xWindow, ownerEvents cInt, eventMask cUint, pointerMode cInt, keyboardMode cInt, confineTo xWindow, cursor cursor, timestamp xTime) cInt {
	return C.XGrabPointer(display, window, ownerEvents, eventMask, pointerMode, keyboardMode, confineTo, cursor, timestamp)
}

func xInternAtom(display *display, name *cChar, onlyIfExists cInt) atom {
	return C.XInternAtom(display, name, onlyIfExists)
}

func xMapWindow(display *display, window xWindow) cInt { return C.XMapWindow(display, window) }

func xMoveResizeWindow(display *display, window xWindow, x cInt, y cInt, width cUint, height cUint) cInt {
	return C.XMoveResizeWindow(display, window, x, y, width, height)
}

func xMoveWindow(display *display, window xWindow, x cInt, y cInt) cInt {
	return C.XMoveWindow(display, window, x, y)
}

func xRaiseWindow(display *display, window xWindow) cInt { return C.XRaiseWindow(display, window) }

func xResizeWindow(display *display, window xWindow, width cUint, height cUint) cInt {
	return C.XResizeWindow(display, window, width, height)
}

func xSelectInput(display *display, window xWindow, mask cLong) cInt {
	return C.XSelectInput(display, window, mask)
}

func xSendEvent(display *display, window xWindow, propagate cInt, mask cLong, event *xEvent) cInt {
	return C.XSendEvent(display, window, propagate, mask, event)
}

func xSetWMNormalHints(display *display, window xWindow, hints *xSizeHints) {
	C.XSetWMNormalHints(display, window, hints)
}

func xTranslateCoordinates(display *display, source xWindow, destination xWindow, x cInt, y cInt, destinationX *cInt, destinationY *cInt, child *xWindow) cInt {
	return C.XTranslateCoordinates(display, source, destination, x, y, destinationX, destinationY, child)
}

func xUngrabPointer(display *display, timestamp xTime) cInt {
	return C.XUngrabPointer(display, timestamp)
}

func xUnmapWindow(display *display, window xWindow) cInt { return C.XUnmapWindow(display, window) }

func xWarpPointer(display *display, source xWindow, destination xWindow, sourceX cInt, sourceY cInt, sourceWidth cUint, sourceHeight cUint, destinationX cInt, destinationY cInt) cInt {
	return C.XWarpPointer(display, source, destination, sourceX, sourceY, sourceWidth, sourceHeight, destinationX, destinationY)
}

func cairoArc(cr *cairoContext, xc cDouble, yc cDouble, radius cDouble, angle1 cDouble, angle2 cDouble) {
	C.cairo_arc(cr, xc, yc, radius, angle1, angle2)
}

func cairoFillPreserve(cr *cairoContext) { C.cairo_fill_preserve(cr) }

func cairoImageSurfaceCreate(format cairoFormat, width, height cInt) *cairoSurface {
	return C.cairo_image_surface_create(format, width, height)
}

func cairoImageSurfaceGetData(surface *cairoSurface) *cUchar {
	return C.cairo_image_surface_get_data(surface)
}

func cairoImageSurfaceGetStride(surface *cairoSurface) cInt {
	return C.cairo_image_surface_get_stride(surface)
}

func cairoLineTo(cr *cairoContext, x cDouble, y cDouble) { C.cairo_line_to(cr, x, y) }

func cairoMoveTo(context *cairoContext, x, y cDouble) {
	C.cairo_move_to(context, x, y)
}

func cairoPaint(context *cairoContext) {
	C.cairo_paint(context)
}

func cairoRestore(context *cairoContext) {
	C.cairo_restore(context)
}

func cairoSave(context *cairoContext) {
	C.cairo_save(context)
}

func cairoScale(context *cairoContext, x, y cDouble) {
	C.cairo_scale(context, x, y)
}

func cairoSetLineWidth(cr *cairoContext, width cDouble) { C.cairo_set_line_width(cr, width) }

func cairoSetSourceRGB(context *cairoContext, r, g, b cDouble) {
	C.cairo_set_source_rgb(context, r, g, b)
}

func cairoSetSourceRgba(cr *cairoContext, red cDouble, green cDouble, blue cDouble, alpha cDouble) {
	C.cairo_set_source_rgba(cr, red, green, blue, alpha)
}

func cairoSetSourceSurface(context *cairoContext, surface *cairoSurface, x, y cDouble) {
	C.cairo_set_source_surface(context, surface, x, y)
}

func cairoStroke(cr *cairoContext) { C.cairo_stroke(cr) }

func cairoSurfaceDestroy(surface *cairoSurface) {
	C.cairo_surface_destroy(surface)
}

func cairoSurfaceMarkDirty(surface *cairoSurface) {
	C.cairo_surface_mark_dirty(surface)
}

func cairoTranslate(context *cairoContext, x, y cDouble) {
	C.cairo_translate(context, x, y)
}

func gASCIIStrcasecmp(a, b *cChar) cInt {
	return C.g_ascii_strcasecmp(a, b)
}

func gActionGetEnabled(action *gAction) gBoolean { return C.g_action_get_enabled(action) }

func gActionGetState(action *gAction) *gVariant { return C.g_action_get_state(action) }

func gActionGetStateType(action *gAction) *gVariantType { return C.g_action_get_state_type(action) }

func gActionGroupActivateAction(action_group *gActionGroup, action_name *gchar, parameter *gVariant) {
	C.g_action_group_activate_action(action_group, action_name, parameter)
}

func gActionMapAddAction(action_map *gActionMap, action *gAction) {
	C.g_action_map_add_action(action_map, action)
}

func gActionMapLookupAction(action_map *gActionMap, action_name *gchar) *gAction {
	return C.g_action_map_lookup_action(action_map, action_name)
}

func gActionMapRemoveAction(action_map *gActionMap, action_name *gchar) {
	C.g_action_map_remove_action(action_map, action_name)
}

func gAppInfoGetDefaultForType(contentType *cChar, mustSupportURIs gBoolean) *gAppInfo {
	return C.g_app_info_get_default_for_type(contentType, mustSupportURIs)
}

func gAppInfoGetDisplayName(info *gAppInfo) *cChar {
	return C.g_app_info_get_display_name(info)
}

func gAppInfoLaunchDefaultForURI(uri *cChar, context *gAppLaunchContext, err **gError) gBoolean {
	return C.g_app_info_launch_default_for_uri(uri, context, err)
}

func gBoxedFree(kind gType, value unsafe.Pointer) { C.g_boxed_free(kind, C.gpointer(value)) }

func gBusGetSync(kind gBusType, cancellable *gCancellable, err **gError) *gDBusConnection {
	return C.g_bus_get_sync(kind, cancellable, err)
}

func gBusUnwatchName(watcher_id cUint) { C.g_bus_unwatch_name(watcher_id) }

func gBusWatchNameOnConnection(connection *gDBusConnection, name *gchar, flags gBusNameWatcherFlags, name_appeared_handler gBusNameAppearedCallback, name_vanished_handler gBusNameVanishedCallback, user_data unsafe.Pointer, user_data_free_func gDestroyNotify) cUint {
	return C.g_bus_watch_name_on_connection(connection, name, flags, name_appeared_handler, name_vanished_handler, C.gpointer(user_data), user_data_free_func)
}

func gBytesGetData(bytes *gBytes, size *gSize) unsafe.Pointer {
	return unsafe.Pointer(C.g_bytes_get_data(bytes, size))
}

func gBytesNew(data unsafe.Pointer, size gSize) *gBytes {
	return C.g_bytes_new(C.gconstpointer(data), size)
}

func gBytesUnref(bytes *gBytes) {
	C.g_bytes_unref(bytes)
}

func gCancellableCancel(cancellable *gCancellable) { C.g_cancellable_cancel(cancellable) }

func gCancellableNew() *gCancellable { return C.g_cancellable_new() }

func gContentTypeGuess(filename *cChar, data *cUchar, size gSize, uncertain *gBoolean) *cChar {
	return C.g_content_type_guess(filename, data, size, uncertain)
}

func gDBusAddressGetForBusSync(bus_type gBusType, cancellable *gCancellable, error **gError) *gchar {
	return C.g_dbus_address_get_for_bus_sync(bus_type, cancellable, error)
}

func gDBusConnectionCallSync(connection *gDBusConnection, busName, objectPath, interfaceName, methodName *cChar, parameters *gVariant, replyType *gVariantType, flags gDBusCallFlags, timeout cInt, cancellable *gCancellable, err **gError) *gVariant {
	return C.g_dbus_connection_call_sync(connection, busName, objectPath, interfaceName, methodName, parameters, replyType, flags, timeout, cancellable, err)
}

func gDBusConnectionCloseSync(connection *gDBusConnection, cancellable *gCancellable, error **gError) gBoolean {
	return C.g_dbus_connection_close_sync(connection, cancellable, error)
}

func gDBusConnectionEmitSignal(connection *gDBusConnection, destination, path, iface, signal *cChar, parameters *gVariant, err **gError) gBoolean {
	return C.g_dbus_connection_emit_signal(connection, destination, path, iface, signal, parameters, err)
}

func gDBusConnectionFlushSync(connection *gDBusConnection, cancellable *gCancellable, err **gError) gBoolean {
	return C.g_dbus_connection_flush_sync(connection, cancellable, err)
}

func gDBusConnectionGetUniqueName(connection *gDBusConnection) *gchar {
	return C.g_dbus_connection_get_unique_name(connection)
}

func gDBusConnectionNewForAddressSync(address *gchar, flags gDBusConnectionFlags, observer *gDBusAuthObserver, cancellable *gCancellable, error **gError) *gDBusConnection {
	return C.g_dbus_connection_new_for_address_sync(address, flags, observer, cancellable, error)
}

func gDBusConnectionRegisterObject(connection *gDBusConnection, object_path *gchar, interface_info *gDBusInterfaceInfo, vtable *gDBusInterfaceVTable, user_data unsafe.Pointer, user_data_free_func gDestroyNotify, error **gError) cUint {
	return C.g_dbus_connection_register_object(connection, object_path, interface_info, vtable, C.gpointer(user_data), user_data_free_func, error)
}

func gDBusConnectionSetExitOnClose(connection *gDBusConnection, exit_on_close gBoolean) {
	C.g_dbus_connection_set_exit_on_close(connection, exit_on_close)
}

func gDBusConnectionSignalSubscribe(connection *gDBusConnection, sender, iface, member, path, arg0 *cChar, flags gDBusSignalFlags, callback gDBusSignalCallback, data unsafe.Pointer, destroy gDestroyNotify) cUint {
	return C.g_dbus_connection_signal_subscribe(connection, sender, iface, member, path, arg0, flags, callback, C.gpointer(data), destroy)
}

func gDBusConnectionUnregisterObject(connection *gDBusConnection, registration_id cUint) gBoolean {
	return C.g_dbus_connection_unregister_object(connection, registration_id)
}

func gDBusMethodInvocationReturnDbusError(invocation *gDBusMethodInvocation, error_name *gchar, error_message *gchar) {
	C.g_dbus_method_invocation_return_dbus_error(invocation, error_name, error_message)
}

func gDBusMethodInvocationReturnValue(invocation *gDBusMethodInvocation, parameters *gVariant) {
	C.g_dbus_method_invocation_return_value(invocation, parameters)
}

func gDBusNodeInfoNewForXml(xml_data *gchar, error **gError) *gDBusNodeInfo {
	return C.g_dbus_node_info_new_for_xml(xml_data, error)
}

func gDBusNodeInfoUnref(info *gDBusNodeInfo) { C.g_dbus_node_info_unref(info) }

func gDateTimeFormat(datetime *gDateTime, format *gchar) *gchar {
	return C.g_date_time_format(datetime, format)
}

func gDateTimeGetDayOfMonth(datetime *gDateTime) gint {
	return C.g_date_time_get_day_of_month(datetime)
}

func gDateTimeGetMonth(datetime *gDateTime) gint { return C.g_date_time_get_month(datetime) }

func gDateTimeGetYear(datetime *gDateTime) gint { return C.g_date_time_get_year(datetime) }

func gDateTimeNewFromUnixUtc(t cInt64) *gDateTime { return C.g_date_time_new_from_unix_utc(t) }

func gDateTimeNewLocal(year gint, month gint, day gint, hour gint, minute gint, seconds gdouble) *gDateTime {
	return C.g_date_time_new_local(year, month, day, hour, minute, seconds)
}

func gDateTimeNewUtc(year gint, month gint, day gint, hour gint, minute gint, seconds gdouble) *gDateTime {
	return C.g_date_time_new_utc(year, month, day, hour, minute, seconds)
}

func gDateTimeUnref(datetime *gDateTime) { C.g_date_time_unref(datetime) }

func gEnumGetValue(enum_class *gEnumClass, value gint) *gEnumValue {
	return C.g_enum_get_value(enum_class, value)
}

func gErrorFree(err *gError) {
	C.g_error_free(err)
}

func gFileGetParent(file *gFile) *gFile {
	return C.g_file_get_parent(file)
}

func gFileGetPath(file *gFile) *cChar {
	return C.g_file_get_path(file)
}

func gFileGetURI(file *gFile) *cChar {
	return C.g_file_get_uri(file)
}

func gFileNewForPath(path *cChar) *gFile {
	return C.g_file_new_for_path(path)
}

func gFileNewForURI(uri *cChar) *gFile {
	return C.g_file_new_for_uri(uri)
}

func gFileQueryFileType(file *gFile, flags gFileQueryInfoFlags, cancellable *gCancellable) gFileType {
	return C.g_file_query_file_type(file, flags, cancellable)
}

func gFree(memory unsafe.Pointer) {
	C.g_free(C.gpointer(memory))
}

func gGetApplicationName() *cChar {
	return C.g_get_application_name()
}

func gGetLanguageNames() **gchar { return C.g_get_language_names() }

func gGetMonotonicTime() cInt64 { return C.g_get_monotonic_time() }

func gGetPrgname() *cChar { return C.g_get_prgname() }

func gIdleAddFull(priority cInt, callback gSourceFunc, data unsafe.Pointer, destroy gDestroyNotify) cUint {
	return C.g_idle_add_full(priority, callback, C.gpointer(data), destroy)
}

func gInputStreamReadBytesAsync(stream *gInputStream, count gSize, priority cInt, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.g_input_stream_read_bytes_async(stream, count, priority, cancellable, callback, C.gpointer(data))
}

func gInputStreamReadBytesFinish(stream *gInputStream, result *gAsyncResult, err **gError) *gBytes {
	return C.g_input_stream_read_bytes_finish(stream, result, err)
}

func gListFree(list *gList) {
	C.g_list_free(list)
}

func gListModelGetItem(model *gListModel, position cUint) unsafe.Pointer {
	return unsafe.Pointer(C.g_list_model_get_item(model, position))
}

func gListModelGetNItems(model *gListModel) cUint { return C.g_list_model_get_n_items(model) }

func gListModelGetType() gType { return C.g_list_model_get_type() }

func gListPrepend(list *gList, data unsafe.Pointer) *gList {
	return C.g_list_prepend(list, (C.gpointer)(unsafe.Pointer(data)))
}

func gListStoreAppend(store *gListStore, item unsafe.Pointer) {
	C.g_list_store_append(store, C.gpointer(item))
}

func gListStoreNew(kind gType) *gListStore { return C.g_list_store_new(kind) }

func gListStoreRemoveAll(store *gListStore) { C.g_list_store_remove_all(store) }

func gListStoreSplice(store *gListStore, position cUint, n_removals cUint, additions *unsafe.Pointer, n_additions cUint) {
	C.g_list_store_splice(store, position, n_removals, (*C.gpointer)(unsafe.Pointer(additions)), n_additions)
}

func gMainContextIteration(context *gMainContext, block gBoolean) gBoolean {
	return C.g_main_context_iteration(context, block)
}

func gMalloc(size gSize) unsafe.Pointer {
	return unsafe.Pointer(C.g_malloc(size))
}

func gMenuAppendItem(menu *gMenu, item *gMenuItem) { C.g_menu_append_item(menu, item) }

func gMenuAppendSection(menu *gMenu, label *gchar, section *gMenuModel) {
	C.g_menu_append_section(menu, label, section)
}

func gMenuAppendSubmenu(menu *gMenu, label *gchar, submenu *gMenuModel) {
	C.g_menu_append_submenu(menu, label, submenu)
}

func gMenuItemNew(label *gchar, detailed_action *gchar) *gMenuItem {
	return C.g_menu_item_new(label, detailed_action)
}

func gMenuItemSetAttributeValue(menu_item *gMenuItem, attribute *gchar, value *gVariant) {
	C.g_menu_item_set_attribute_value(menu_item, attribute, value)
}

func gMenuItemSetIcon(menu_item *gMenuItem, icon *gIcon) { C.g_menu_item_set_icon(menu_item, icon) }

func gMenuNew() *gMenu { return C.g_menu_new() }

func gMenuRemoveAll(menu *gMenu) { C.g_menu_remove_all(menu) }

func gModuleOpen(file_name *gchar, flags gModuleFlags) *gModule {
	return C.g_module_open(file_name, flags)
}

func gModuleSymbol(module *gModule, symbol_name *gchar, symbol *unsafe.Pointer) gBoolean {
	return C.g_module_symbol(module, symbol_name, (*C.gpointer)(unsafe.Pointer(symbol)))
}

func gObjectClassFindProperty(oclass *gObjectClass, property_name *gchar) *gParamSpec {
	return C.g_object_class_find_property(oclass, property_name)
}

func gObjectGetProperty(object *gObject, property_name *gchar, value *gValue) {
	C.g_object_get_property(object, property_name, value)
}

func gObjectGetQdata(object *gObject, quark gQuark) unsafe.Pointer {
	return unsafe.Pointer(C.g_object_get_qdata(object, quark))
}

func gObjectGetType() gType { return C.g_object_get_type() }

func gObjectNewWithProperties(kind gType, count cUint, names **cChar, values *gValue) *gObject {
	return C.g_object_new_with_properties(kind, count, names, values)
}

func gObjectRef(object unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(C.g_object_ref(C.gpointer(object)))
}

func gObjectRefSink(object unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(C.g_object_ref_sink(C.gpointer(object)))
}

func gObjectSetProperty(object *gObject, name *cChar, value *gValue) {
	C.g_object_set_property(object, name, value)
}

func gObjectSetQdataFull(object *gObject, quark gQuark, data unsafe.Pointer, destroy gDestroyNotify) {
	C.g_object_set_qdata_full(object, quark, C.gpointer(data), destroy)
}

func gObjectUnref(object unsafe.Pointer) {
	C.g_object_unref(C.gpointer(object))
}

func gQuarkFromString(string *gchar) gQuark { return C.g_quark_from_string(string) }

func gSListFree(list *gSList) { C.g_slist_free(list) }

func gSettingsGetString(settings *gSettings, key *gchar) *gchar {
	return C.g_settings_get_string(settings, key)
}

func gSettingsNew(schema_id *gchar) *gSettings { return C.g_settings_new(schema_id) }

func gSettingsSchemaHasKey(schema *gSettingsSchema, name *gchar) gBoolean {
	return C.g_settings_schema_has_key(schema, name)
}

func gSettingsSchemaSourceGetDefault() *gSettingsSchemaSource {
	return C.g_settings_schema_source_get_default()
}

func gSettingsSchemaSourceLookup(source *gSettingsSchemaSource, schema_id *gchar, recursive gBoolean) *gSettingsSchema {
	return C.g_settings_schema_source_lookup(source, schema_id, recursive)
}

func gSettingsSchemaUnref(schema *gSettingsSchema) { C.g_settings_schema_unref(schema) }

func gSignalConnectData(instance unsafe.Pointer, signal *cChar, callback gCallback, data unsafe.Pointer, destroy gClosureNotify, flags gConnectFlags) gUlong {
	return C.g_signal_connect_data(C.gpointer(instance), signal, callback, C.gpointer(data), destroy, flags)
}

func gSignalHandlerDisconnect(instance unsafe.Pointer, handler gUlong) {
	C.g_signal_handler_disconnect(C.gpointer(instance), handler)
}

func gSignalStopEmissionByName(instance unsafe.Pointer, detailed_signal *gchar) {
	C.g_signal_stop_emission_by_name((C.gpointer)(unsafe.Pointer(instance)), detailed_signal)
}

func gSimpleActionGroupNew() *gSimpleActionGroup { return C.g_simple_action_group_new() }

func gSimpleActionNew(name *gchar, parameter_type *gVariantType) *gSimpleAction {
	return C.g_simple_action_new(name, parameter_type)
}

func gSimpleActionNewStateful(name *gchar, parameter_type *gVariantType, state *gVariant) *gSimpleAction {
	return C.g_simple_action_new_stateful(name, parameter_type, state)
}

func gSimpleActionSetEnabled(simple *gSimpleAction, enabled gBoolean) {
	C.g_simple_action_set_enabled(simple, enabled)
}

func gSimpleActionSetState(simple *gSimpleAction, value *gVariant) {
	C.g_simple_action_set_state(simple, value)
}

func gSourceRemove(id cUint) gBoolean { return C.g_source_remove(id) }

func gStrdup(value *cChar) *cChar { return C.g_strdup(value) }

func gThemedIconNew(iconname *cChar) *gIcon { return C.g_themed_icon_new(iconname) }

func gTimeoutAddFull(priority cInt, interval cUint, callback gSourceFunc, data unsafe.Pointer, destroy gDestroyNotify) cUint {
	return C.g_timeout_add_full(priority, interval, callback, C.gpointer(data), destroy)
}

func gTypeCheckInstanceIsA(instance *gTypeInstance, iface_type gType) gBoolean {
	return C.g_type_check_instance_is_a(instance, iface_type)
}

func gTypeClassRef(arg0 gType) unsafe.Pointer { return unsafe.Pointer(C.g_type_class_ref(arg0)) }

func gTypeClassUnref(g_class unsafe.Pointer) {
	C.g_type_class_unref((C.gpointer)(unsafe.Pointer(g_class)))
}

func gTypeFundamental(type_id gType) gType { return C.g_type_fundamental(type_id) }

func gTypeRegisterStaticSimple(parent_type gType, type_name *gchar, class_size cUint, class_init gClassInitFunc, instance_size cUint, instance_init gInstanceInitFunc, flags gTypeFlags) gType {
	return C.g_type_register_static_simple(parent_type, type_name, class_size, class_init, instance_size, instance_init, flags)
}

func gValueGetBoolean(value *gValue) gBoolean { return C.g_value_get_boolean(value) }

func gValueGetBoxed(value *gValue) unsafe.Pointer { return unsafe.Pointer(C.g_value_get_boxed(value)) }

func gValueGetEnum(value *gValue) gint { return C.g_value_get_enum(value) }

func gValueGetFlags(value *gValue) cUint { return C.g_value_get_flags(value) }

func gValueGetInt(value *gValue) gint { return C.g_value_get_int(value) }

func gValueGetObject(value *gValue) unsafe.Pointer {
	return unsafe.Pointer(C.g_value_get_object(value))
}

func gValueGetString(value *gValue) *gchar { return C.g_value_get_string(value) }

func gValueGetUint(value *gValue) cUint { return C.g_value_get_uint(value) }

func gValueInit(value *gValue, kind gType) *gValue { return C.g_value_init(value, kind) }

func gValueSetBoolean(value *gValue, boolean gBoolean) { C.g_value_set_boolean(value, boolean) }

func gValueSetBoxed(value *gValue, boxed unsafe.Pointer) {
	C.g_value_set_boxed(value, C.gconstpointer(boxed))
}

func gValueSetDouble(value *gValue, v_double gdouble) { C.g_value_set_double(value, v_double) }

func gValueSetEnum(value *gValue, number cInt) { C.g_value_set_enum(value, number) }

func gValueSetInt(value *gValue, v_int gint) { C.g_value_set_int(value, v_int) }

func gValueSetObject(value *gValue, object unsafe.Pointer) {
	C.g_value_set_object(value, C.gpointer(object))
}

func gValueSetPointer(value *gValue, v_pointer unsafe.Pointer) {
	C.g_value_set_pointer(value, (C.gpointer)(unsafe.Pointer(v_pointer)))
}

func gValueSetString(value *gValue, text *cChar) { C.g_value_set_string(value, text) }

func gValueUnset(value *gValue) { C.g_value_unset(value) }

func gVariantGetBoolean(value *gVariant) gBoolean { return C.g_variant_get_boolean(value) }

func gVariantGetChildValue(value *gVariant, index gSize) *gVariant {
	return C.g_variant_get_child_value(value, index)
}

func gVariantGetInt32(value *gVariant) gint32 { return C.g_variant_get_int32(value) }

func gVariantGetString(value *gVariant, size *gSize) *cChar {
	return C.g_variant_get_string(value, size)
}

func gVariantGetUint32(value *gVariant) cUint32 { return C.g_variant_get_uint32(value) }

func gVariantGetVariant(value *gVariant) *gVariant { return C.g_variant_get_variant(value) }

func gVariantNChildren(value *gVariant) gSize { return C.g_variant_n_children(value) }

func gVariantNewArray(elementType *gVariantType, children **gVariant, count gSize) *gVariant {
	return C.g_variant_new_array(elementType, children, count)
}

func gVariantNewBoolean(value gBoolean) *gVariant { return C.g_variant_new_boolean(value) }

func gVariantNewDictEntry(key, value *gVariant) *gVariant {
	return C.g_variant_new_dict_entry(key, value)
}

func gVariantNewFixedArray(element_type *gVariantType, elements unsafe.Pointer, n_elements gSize, element_size gSize) *gVariant {
	return C.g_variant_new_fixed_array(element_type, C.gconstpointer(elements), n_elements, element_size)
}

func gVariantNewInt32(value cInt) *gVariant { return C.g_variant_new_int32(value) }

func gVariantNewInt64(value cInt64) *gVariant { return C.g_variant_new_int64(value) }

func gVariantNewObjectPath(object_path *gchar) *gVariant {
	return C.g_variant_new_object_path(object_path)
}

func gVariantNewString(value *cChar) *gVariant {
	return C.g_variant_new_string(value)
}

func gVariantNewTuple(children **gVariant, count gSize) *gVariant {
	return C.g_variant_new_tuple(children, count)
}

func gVariantNewUint32(value cUint32) *gVariant { return C.g_variant_new_uint32(value) }

func gVariantNewVariant(value *gVariant) *gVariant { return C.g_variant_new_variant(value) }

func gVariantRef(value *gVariant) *gVariant { return C.g_variant_ref(value) }

func gVariantRefSink(value *gVariant) *gVariant { return C.g_variant_ref_sink(value) }

func gVariantTypeFree(kind *gVariantType) { C.g_variant_type_free(kind) }

func gVariantTypeNew(signature *cChar) *gVariantType { return C.g_variant_type_new(signature) }

func gVariantUnref(value *gVariant) {
	C.g_variant_unref(value)
}

func gdkButtonEventGetButton(event *gdkEvent) cUint { return C.gdk_button_event_get_button(event) }

func gdkClipboardGetFormats(clipboard *gdkClipboard) *gdkContentFormats {
	return C.gdk_clipboard_get_formats(clipboard)
}

func gdkClipboardReadAsync(clipboard *gdkClipboard, mimes **cChar, priority cInt, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gdk_clipboard_read_async(clipboard, mimes, priority, cancellable, callback, C.gpointer(data))
}

func gdkClipboardReadFinish(clipboard *gdkClipboard, result *gAsyncResult, mime **cChar, err **gError) *gInputStream {
	return C.gdk_clipboard_read_finish(clipboard, result, mime, err)
}

func gdkClipboardReadTextAsync(clipboard *gdkClipboard, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gdk_clipboard_read_text_async(clipboard, cancellable, callback, C.gpointer(data))
}

func gdkClipboardReadTextFinish(clipboard *gdkClipboard, result *gAsyncResult, err **gError) *cChar {
	return C.gdk_clipboard_read_text_finish(clipboard, result, err)
}

func gdkClipboardReadTextureAsync(clipboard *gdkClipboard, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gdk_clipboard_read_texture_async(clipboard, cancellable, callback, C.gpointer(data))
}

func gdkClipboardReadTextureFinish(clipboard *gdkClipboard, result *gAsyncResult, err **gError) *gdkTexture {
	return C.gdk_clipboard_read_texture_finish(clipboard, result, err)
}

func gdkClipboardReadValueAsync(clipboard *gdkClipboard, kind gType, priority cInt, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gdk_clipboard_read_value_async(clipboard, kind, priority, cancellable, callback, C.gpointer(data))
}

func gdkClipboardReadValueFinish(clipboard *gdkClipboard, result *gAsyncResult, err **gError) *gValue {
	return C.gdk_clipboard_read_value_finish(clipboard, result, err)
}

func gdkClipboardSetContent(clipboard *gdkClipboard, provider *gdkContentProvider) gBoolean {
	return C.gdk_clipboard_set_content(clipboard, provider)
}

func gdkContentFormatsBuilderAddGType(builder *gdkContentFormatsBuilder, arg1 gType) {
	C.gdk_content_formats_builder_add_gtype(builder, arg1)
}

func gdkContentFormatsBuilderAddMimeType(builder *gdkContentFormatsBuilder, mime_type *cChar) {
	C.gdk_content_formats_builder_add_mime_type(builder, mime_type)
}

func gdkContentFormatsBuilderFreeToFormats(builder *gdkContentFormatsBuilder) *gdkContentFormats {
	return C.gdk_content_formats_builder_free_to_formats(builder)
}

func gdkContentFormatsBuilderNew() *gdkContentFormatsBuilder {
	return C.gdk_content_formats_builder_new()
}

func gdkContentFormatsContainGType(formats *gdkContentFormats, kind gType) gBoolean {
	return C.gdk_content_formats_contain_gtype(formats, kind)
}

func gdkContentFormatsContainMimeType(formats *gdkContentFormats, mime *cChar) gBoolean {
	return C.gdk_content_formats_contain_mime_type(formats, mime)
}

func gdkContentFormatsGetMimeTypes(formats *gdkContentFormats, count *gSize) **cChar {
	return C.gdk_content_formats_get_mime_types(formats, count)
}

func gdkContentProviderNewForBytes(mime *cChar, bytes *gBytes) *gdkContentProvider {
	return C.gdk_content_provider_new_for_bytes(mime, bytes)
}

func gdkContentProviderNewForValue(value *gValue) *gdkContentProvider {
	return C.gdk_content_provider_new_for_value(value)
}

func gdkContentProviderNewUnion(providers **gdkContentProvider, count gSize) *gdkContentProvider {
	return C.gdk_content_provider_new_union(providers, count)
}

func gdkCrossingEventGetDetail(event *gdkEvent) gdkNotifyType {
	return C.gdk_crossing_event_get_detail(event)
}

func gdkDeviceGetName(device *gdkDevice) *cChar { return C.gdk_device_get_name(device) }

func gdkDeviceGetSource(device *gdkDevice) gdkInputSource { return C.gdk_device_get_source(device) }

func gdkDeviceToolGetToolType(tool *gdkDeviceTool) gdkDeviceToolType {
	return C.gdk_device_tool_get_tool_type(tool)
}

func gdkDisplayGetClipboard(display *gdkDisplay) *gdkClipboard {
	return C.gdk_display_get_clipboard(display)
}

func gdkDisplayGetDefault() *gdkDisplay {
	return C.gdk_display_get_default()
}

func gdkDisplayGetDefaultSeat(display *gdkDisplay) *gdkSeat {
	return C.gdk_display_get_default_seat(display)
}

func gdkDisplayGetMonitorAtSurface(display *gdkDisplay, surface *gdkSurface) *gdkMonitor {
	return C.gdk_display_get_monitor_at_surface(display, surface)
}

func gdkDisplayGetMonitors(self *gdkDisplay) *gListModel { return C.gdk_display_get_monitors(self) }

func gdkDropFinish(self *gdkDrop, action gdkDragAction) { C.gdk_drop_finish(self, action) }

func gdkDropGetActions(self *gdkDrop) gdkDragAction { return C.gdk_drop_get_actions(self) }

func gdkDropGetFormats(self *gdkDrop) *gdkContentFormats { return C.gdk_drop_get_formats(self) }

func gdkDropReadAsync(self *gdkDrop, mime_types **cChar, io_priority cInt, cancellable *gCancellable, callback gAsyncReadyCallback, user_data unsafe.Pointer) {
	C.gdk_drop_read_async(self, mime_types, io_priority, cancellable, callback, (C.gpointer)(unsafe.Pointer(user_data)))
}

func gdkDropReadFinish(self *gdkDrop, result *gAsyncResult, out_mime_type **cChar, error **gError) *gInputStream {
	return C.gdk_drop_read_finish(self, result, out_mime_type, error)
}

func gdkDropReadValueAsync(self *gdkDrop, arg1 gType, io_priority cInt, cancellable *gCancellable, callback gAsyncReadyCallback, user_data unsafe.Pointer) {
	C.gdk_drop_read_value_async(self, arg1, io_priority, cancellable, callback, (C.gpointer)(unsafe.Pointer(user_data)))
}

func gdkDropReadValueFinish(self *gdkDrop, result *gAsyncResult, error **gError) *gValue {
	return C.gdk_drop_read_value_finish(self, result, error)
}

func gdkEventGetAxis(event *gdkEvent, axis_use gdkAxisUse, value *cDouble) gBoolean {
	return C.gdk_event_get_axis(event, axis_use, value)
}

func gdkEventGetDevice(event *gdkEvent) *gdkDevice { return C.gdk_event_get_device(event) }

func gdkEventGetDeviceTool(event *gdkEvent) *gdkDeviceTool { return C.gdk_event_get_device_tool(event) }

func gdkEventGetEventSequence(event *gdkEvent) *gdkEventSequence {
	return C.gdk_event_get_event_sequence(event)
}

func gdkEventGetEventType(event *gdkEvent) gdkEventType { return C.gdk_event_get_event_type(event) }

func gdkEventGetModifierState(event *gdkEvent) gdkModifierType {
	return C.gdk_event_get_modifier_state(event)
}

func gdkEventGetPosition(event *gdkEvent, x *cDouble, y *cDouble) gBoolean {
	return C.gdk_event_get_position(event, x, y)
}

func gdkFileListGetFiles(files *gdkFileList) *gSList { return C.gdk_file_list_get_files(files) }

func gdkFileListGetType() gType { return C.gdk_file_list_get_type() }

func gdkFileListNewFromArray(files **gFile, count gSize) *gdkFileList {
	return C.gdk_file_list_new_from_array(files, count)
}

func gdkFrameClockGetFrameTime(frame_clock *gdkFrameClock) cInt64 {
	return C.gdk_frame_clock_get_frame_time(frame_clock)
}

func gdkFrameClockGetRefreshInfo(frame_clock *gdkFrameClock, base_time cInt64, refresh_interval_return *cInt64, presentation_time_return *cInt64) {
	C.gdk_frame_clock_get_refresh_info(frame_clock, base_time, refresh_interval_return, presentation_time_return)
}

func gdkKeyEventGetKeycode(event *gdkEvent) cUint { return C.gdk_key_event_get_keycode(event) }

func gdkKeyEventGetKeyval(event *gdkEvent) cUint { return C.gdk_key_event_get_keyval(event) }

func gdkKeyvalToLower(keyval cUint) cUint { return C.gdk_keyval_to_lower(keyval) }

func gdkKeyvalToUnicode(keyval cUint) cUint32 { return C.gdk_keyval_to_unicode(keyval) }

func gdkMemoryTextureNew(width, height cInt, format gdkMemoryFormat, bytes *gBytes, stride gSize) *gdkTexture {
	return C.gdk_memory_texture_new(width, height, format, bytes, stride)
}

func gdkMonitorGetConnector(monitor *gdkMonitor) *cChar { return C.gdk_monitor_get_connector(monitor) }

func gdkMonitorGetDescription(monitor *gdkMonitor) *cChar {
	return C.gdk_monitor_get_description(monitor)
}

func gdkMonitorGetGeometry(monitor *gdkMonitor, geometry *gdkRectangle) {
	C.gdk_monitor_get_geometry(monitor, geometry)
}

func gdkMonitorGetRefreshRate(monitor *gdkMonitor) cInt {
	return C.gdk_monitor_get_refresh_rate(monitor)
}

func gdkMonitorGetScale(monitor *gdkMonitor) cDouble { return C.gdk_monitor_get_scale(monitor) }

func gdkMonitorGetWidthMm(monitor *gdkMonitor) cInt { return C.gdk_monitor_get_width_mm(monitor) }

func gdkPaintableGetIntrinsicHeight(paintable *gdkPaintable) cInt {
	return C.gdk_paintable_get_intrinsic_height(paintable)
}

func gdkPaintableGetIntrinsicWidth(paintable *gdkPaintable) cInt {
	return C.gdk_paintable_get_intrinsic_width(paintable)
}

func gdkRgbaGetType() gType { return C.gdk_rgba_get_type() }

func gdkScrollEventGetDeltas(event *gdkEvent, delta_x *cDouble, delta_y *cDouble) {
	C.gdk_scroll_event_get_deltas(event, delta_x, delta_y)
}

func gdkScrollEventGetDirection(event *gdkEvent) gdkScrollDirection {
	return C.gdk_scroll_event_get_direction(event)
}

func gdkSeatGetPointer(seat *gdkSeat) *gdkDevice { return C.gdk_seat_get_pointer(seat) }

func gdkSetAllowedBackends(backends *cChar) { C.gdk_set_allowed_backends(backends) }

func gdkSurfaceGetDevicePosition(surface *gdkSurface, device *gdkDevice, x *cDouble, y *cDouble, mask *gdkModifierType) gBoolean {
	return C.gdk_surface_get_device_position(surface, device, x, y, mask)
}

func gdkSurfaceGetDisplay(surface *gdkSurface) *gdkDisplay { return C.gdk_surface_get_display(surface) }

func gdkTextureDownloaderDownloadInto(downloader *gdkTextureDownloader, data *cUchar, stride gSize) {
	C.gdk_texture_downloader_download_into(downloader, data, stride)
}

func gdkTextureDownloaderFree(downloader *gdkTextureDownloader) {
	C.gdk_texture_downloader_free(downloader)
}

func gdkTextureDownloaderNew(texture *gdkTexture) *gdkTextureDownloader {
	return C.gdk_texture_downloader_new(texture)
}

func gdkTextureDownloaderSetFormat(downloader *gdkTextureDownloader, format gdkMemoryFormat) {
	C.gdk_texture_downloader_set_format(downloader, format)
}

func gdkTextureGetHeight(texture *gdkTexture) cInt { return C.gdk_texture_get_height(texture) }

func gdkTextureGetType() gType { return C.gdk_texture_get_type() }

func gdkTextureGetWidth(texture *gdkTexture) cInt { return C.gdk_texture_get_width(texture) }

func gdkTextureNewFromBytes(bytes *gBytes, error **gError) *gdkTexture {
	return C.gdk_texture_new_from_bytes(bytes, error)
}

func gdkToplevelGetState(toplevel *gdkToplevel) gdkToplevelState {
	return C.gdk_toplevel_get_state(toplevel)
}

func gdkTouchpadEventGetDeltas(event *gdkEvent, dx *cDouble, dy *cDouble) {
	C.gdk_touchpad_event_get_deltas(event, dx, dy)
}

func gdkTouchpadEventGetGesturePhase(event *gdkEvent) gdkTouchpadGesturePhase {
	return C.gdk_touchpad_event_get_gesture_phase(event)
}

func gdkTouchpadEventGetPinchAngleDelta(event *gdkEvent) cDouble {
	return C.gdk_touchpad_event_get_pinch_angle_delta(event)
}

func gdkTouchpadEventGetPinchScale(event *gdkEvent) cDouble {
	return C.gdk_touchpad_event_get_pinch_scale(event)
}

func gdkX11DisplayGetXdisplay(display *gdkDisplay) *display {
	return C.gdk_x11_display_get_xdisplay(display)
}

func gdkX11MonitorGetType() gType { return C.gdk_x11_monitor_get_type() }

func gdkX11MonitorGetWorkarea(monitor *gdkMonitor, workarea *gdkRectangle) {
	C.gdk_x11_monitor_get_workarea(monitor, workarea)
}

func gdkX11SurfaceGetType() gType { return C.gdk_x11_surface_get_type() }

func gdkX11SurfaceGetXid(surface *gdkSurface) xWindow { return C.gdk_x11_surface_get_xid(surface) }

func gskTransformScale(next *gskTransform, factor_x cFloat, factor_y cFloat) *gskTransform {
	return C.gsk_transform_scale(next, factor_x, factor_y)
}

func gskTransformTranslate(next *gskTransform, point *graphenePoint) *gskTransform {
	return C.gsk_transform_translate(next, point)
}

func gtkAboutDialogNew() *gtkWidget {
	return C.gtk_about_dialog_new()
}

func gtkAboutDialogSetComments(dialog *gtkAboutDialog, comments *cChar) {
	C.gtk_about_dialog_set_comments(dialog, comments)
}

func gtkAboutDialogSetCopyright(dialog *gtkAboutDialog, copyright *cChar) {
	C.gtk_about_dialog_set_copyright(dialog, copyright)
}

func gtkAboutDialogSetLogo(dialog *gtkAboutDialog, logo *gdkPaintable) {
	C.gtk_about_dialog_set_logo(dialog, logo)
}

func gtkAboutDialogSetProgramName(dialog *gtkAboutDialog, name *cChar) {
	C.gtk_about_dialog_set_program_name(dialog, name)
}

func gtkAboutDialogSetVersion(dialog *gtkAboutDialog, version *cChar) {
	C.gtk_about_dialog_set_version(dialog, version)
}

func gtkAboutDialogSetWebsite(dialog *gtkAboutDialog, website *cChar) {
	C.gtk_about_dialog_set_website(dialog, website)
}

func gtkAccessibleAnnounce(self *gtkAccessible, message *cChar, priority gtkAccessibleAnnouncementPriority) {
	C.gtk_accessible_announce(self, message, priority)
}

func gtkAccessibleGetAccessibleRole(self *gtkAccessible) gtkAccessibleRole {
	return C.gtk_accessible_get_accessible_role(self)
}

func gtkAccessibleGetFirstAccessibleChild(self *gtkAccessible) *gtkAccessible {
	return C.gtk_accessible_get_first_accessible_child(self)
}

func gtkAccessibleGetNextAccessibleSibling(self *gtkAccessible) *gtkAccessible {
	return C.gtk_accessible_get_next_accessible_sibling(self)
}

func gtkAccessibleRelationInitValue(relation gtkAccessibleRelation, value *gValue) {
	C.gtk_accessible_relation_init_value(relation, value)
}

func gtkAccessibleResetProperty(self *gtkAccessible, property gtkAccessibleProperty) {
	C.gtk_accessible_reset_property(self, property)
}

func gtkAccessibleResetRelation(self *gtkAccessible, relation gtkAccessibleRelation) {
	C.gtk_accessible_reset_relation(self, relation)
}

func gtkAccessibleRoleGetType() gType { return C.gtk_accessible_role_get_type() }

func gtkAccessibleStateInitValue(state gtkAccessibleState, value *gValue) {
	C.gtk_accessible_state_init_value(state, value)
}

func gtkAccessibleUpdatePropertyValue(accessible *gtkAccessible, count cInt, properties *gtkAccessibleProperty, values *gValue) {
	C.gtk_accessible_update_property_value(accessible, count, properties, values)
}

func gtkAccessibleUpdateRelationValue(self *gtkAccessible, n_relations cInt, relations *gtkAccessibleRelation, values *gValue) {
	C.gtk_accessible_update_relation_value(self, n_relations, relations, values)
}

func gtkAccessibleUpdateStateValue(self *gtkAccessible, n_states cInt, states *gtkAccessibleState, values *gValue) {
	C.gtk_accessible_update_state_value(self, n_states, states, values)
}

func gtkActivateActionGet() *gtkShortcutAction { return C.gtk_activate_action_get() }

func gtkAdjustmentConfigure(adjustment *gtkAdjustment, value cDouble, lower cDouble, upper cDouble, step_increment cDouble, page_increment cDouble, page_size cDouble) {
	C.gtk_adjustment_configure(adjustment, value, lower, upper, step_increment, page_increment, page_size)
}

func gtkAdjustmentGetLower(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_lower(adjustment)
}

func gtkAdjustmentGetPageIncrement(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_page_increment(adjustment)
}

func gtkAdjustmentGetPageSize(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_page_size(adjustment)
}

func gtkAdjustmentGetStepIncrement(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_step_increment(adjustment)
}

func gtkAdjustmentGetUpper(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_upper(adjustment)
}

func gtkAdjustmentGetValue(adjustment *gtkAdjustment) cDouble {
	return C.gtk_adjustment_get_value(adjustment)
}

func gtkAdjustmentNew(value cDouble, lower cDouble, upper cDouble, step_increment cDouble, page_increment cDouble, page_size cDouble) *gtkAdjustment {
	return C.gtk_adjustment_new(value, lower, upper, step_increment, page_increment, page_size)
}

func gtkAdjustmentSetValue(adjustment *gtkAdjustment, value cDouble) {
	C.gtk_adjustment_set_value(adjustment, value)
}

func gtkBitsetAdd(self *gtkBitset, value cUint) gBoolean { return C.gtk_bitset_add(self, value) }

func gtkBitsetNewEmpty() *gtkBitset { return C.gtk_bitset_new_empty() }

func gtkBitsetNewRange(start cUint, n_items cUint) *gtkBitset {
	return C.gtk_bitset_new_range(start, n_items)
}

func gtkBitsetUnref(self *gtkBitset) { C.gtk_bitset_unref(self) }

func gtkBoxAppend(box *gtkBox, child *gtkWidget) { C.gtk_box_append(box, child) }

func gtkBoxGetType() gType { return C.gtk_box_get_type() }

func gtkBoxNew(orientation gtkOrientation, spacing cInt) *gtkWidget {
	return C.gtk_box_new(orientation, spacing)
}

func gtkBoxRemove(box *gtkBox, child *gtkWidget) { C.gtk_box_remove(box, child) }

func gtkBoxReorderChildAfter(box *gtkBox, child *gtkWidget, sibling *gtkWidget) {
	C.gtk_box_reorder_child_after(box, child, sibling)
}

func gtkBoxSetSpacing(box *gtkBox, spacing cInt) { C.gtk_box_set_spacing(box, spacing) }

func gtkButtonGetLabel(button *gtkButton) *cChar { return C.gtk_button_get_label(button) }

func gtkButtonGetType() gType { return C.gtk_button_get_type() }

func gtkButtonNew() *gtkWidget { return C.gtk_button_new() }

func gtkButtonNewFromIconName(icon_name *cChar) *gtkWidget {
	return C.gtk_button_new_from_icon_name(icon_name)
}

func gtkButtonNewWithLabel(label *cChar) *gtkWidget { return C.gtk_button_new_with_label(label) }

func gtkButtonSetChild(button *gtkButton, child *gtkWidget) { C.gtk_button_set_child(button, child) }

func gtkButtonSetLabel(button *gtkButton, label *cChar) { C.gtk_button_set_label(button, label) }

func gtkCSSProviderLoadFromString(css_provider *gtkCssProvider, string *cChar) {
	C.gtk_css_provider_load_from_string(css_provider, string)
}

func gtkCSSProviderNew() *gtkCssProvider { return C.gtk_css_provider_new() }

func gtkCalendarClearMarks(calendar *gtkCalendar) { C.gtk_calendar_clear_marks(calendar) }

func gtkCalendarGetDate(self *gtkCalendar) *gDateTime { return C.gtk_calendar_get_date(self) }

func gtkCalendarMarkDay(calendar *gtkCalendar, day cUint) { C.gtk_calendar_mark_day(calendar, day) }

func gtkCalendarNew() *gtkWidget { return C.gtk_calendar_new() }

func gtkCalendarSelectDay(self *gtkCalendar, date *gDateTime) { C.gtk_calendar_select_day(self, date) }

func gtkCalendarSetShowDayNames(self *gtkCalendar, value gBoolean) {
	C.gtk_calendar_set_show_day_names(self, value)
}

func gtkCenterBoxGetCenterWidget(self *gtkCenterBox) *gtkWidget {
	return C.gtk_center_box_get_center_widget(self)
}

func gtkCenterBoxGetEndWidget(self *gtkCenterBox) *gtkWidget {
	return C.gtk_center_box_get_end_widget(self)
}

func gtkCenterBoxGetStartWidget(self *gtkCenterBox) *gtkWidget {
	return C.gtk_center_box_get_start_widget(self)
}

func gtkCenterBoxNew() *gtkWidget { return C.gtk_center_box_new() }

func gtkCenterBoxSetCenterWidget(self *gtkCenterBox, child *gtkWidget) {
	C.gtk_center_box_set_center_widget(self, child)
}

func gtkCenterBoxSetEndWidget(self *gtkCenterBox, child *gtkWidget) {
	C.gtk_center_box_set_end_widget(self, child)
}

func gtkCenterBoxSetStartWidget(self *gtkCenterBox, child *gtkWidget) {
	C.gtk_center_box_set_start_widget(self, child)
}

func gtkCheckButtonGetActive(self *gtkCheckButton) gBoolean {
	return C.gtk_check_button_get_active(self)
}

func gtkCheckButtonGetInconsistent(check_button *gtkCheckButton) gBoolean {
	return C.gtk_check_button_get_inconsistent(check_button)
}

func gtkCheckButtonGetLabel(self *gtkCheckButton) *cChar { return C.gtk_check_button_get_label(self) }

func gtkCheckButtonGetType() gType { return C.gtk_check_button_get_type() }

func gtkCheckButtonNewWithLabel(label *cChar) *gtkWidget {
	return C.gtk_check_button_new_with_label(label)
}

func gtkCheckButtonSetActive(self *gtkCheckButton, setting gBoolean) {
	C.gtk_check_button_set_active(self, setting)
}

func gtkCheckButtonSetChild(button *gtkCheckButton, child *gtkWidget) {
	C.gtk_check_button_set_child(button, child)
}

func gtkCheckButtonSetGroup(self *gtkCheckButton, group *gtkCheckButton) {
	C.gtk_check_button_set_group(self, group)
}

func gtkCheckButtonSetInconsistent(check_button *gtkCheckButton, inconsistent gBoolean) {
	C.gtk_check_button_set_inconsistent(check_button, inconsistent)
}

func gtkCheckButtonSetLabel(self *gtkCheckButton, label *cChar) {
	C.gtk_check_button_set_label(self, label)
}

func gtkColorDialogButtonGetRgba(self *gtkColorDialogButton) *gdkRGBA {
	return C.gtk_color_dialog_button_get_rgba(self)
}

func gtkColorDialogButtonNew(dialog *gtkColorDialog) *gtkWidget {
	return C.gtk_color_dialog_button_new(dialog)
}

func gtkColorDialogButtonSetRgba(self *gtkColorDialogButton, color *gdkRGBA) {
	C.gtk_color_dialog_button_set_rgba(self, color)
}

func gtkColorDialogNew() *gtkColorDialog { return C.gtk_color_dialog_new() }

func gtkColorDialogSetWithAlpha(self *gtkColorDialog, with_alpha gBoolean) {
	C.gtk_color_dialog_set_with_alpha(self, with_alpha)
}

func gtkColumnViewAppendColumn(self *gtkColumnView, column *gtkColumnViewColumn) {
	C.gtk_column_view_append_column(self, column)
}

func gtkColumnViewColumnGetId(self *gtkColumnViewColumn) *cChar {
	return C.gtk_column_view_column_get_id(self)
}

func gtkColumnViewColumnNew(title *cChar, factory *gtkListItemFactory) *gtkColumnViewColumn {
	return C.gtk_column_view_column_new(title, factory)
}

func gtkColumnViewColumnSetExpand(self *gtkColumnViewColumn, expand gBoolean) {
	C.gtk_column_view_column_set_expand(self, expand)
}

func gtkColumnViewColumnSetFixedWidth(self *gtkColumnViewColumn, fixed_width cInt) {
	C.gtk_column_view_column_set_fixed_width(self, fixed_width)
}

func gtkColumnViewColumnSetId(self *gtkColumnViewColumn, id *cChar) {
	C.gtk_column_view_column_set_id(self, id)
}

func gtkColumnViewColumnSetResizable(self *gtkColumnViewColumn, resizable gBoolean) {
	C.gtk_column_view_column_set_resizable(self, resizable)
}

func gtkColumnViewColumnSetSorter(self *gtkColumnViewColumn, sorter *gtkSorter) {
	C.gtk_column_view_column_set_sorter(self, sorter)
}

func gtkColumnViewGetSorter(self *gtkColumnView) *gtkSorter {
	return C.gtk_column_view_get_sorter(self)
}

func gtkColumnViewNew(model *gtkSelectionModel) *gtkWidget { return C.gtk_column_view_new(model) }

func gtkColumnViewScrollTo(self *gtkColumnView, pos cUint, column *gtkColumnViewColumn, flags gtkListScrollFlags, scroll *gtkScrollInfo) {
	C.gtk_column_view_scroll_to(self, pos, column, flags, scroll)
}

func gtkColumnViewSetReorderable(self *gtkColumnView, reorderable gBoolean) {
	C.gtk_column_view_set_reorderable(self, reorderable)
}

func gtkColumnViewSetShowRowSeparators(self *gtkColumnView, show_row_separators gBoolean) {
	C.gtk_column_view_set_show_row_separators(self, show_row_separators)
}

func gtkColumnViewSorterGetPrimarySortColumn(self *gtkColumnViewSorter) *gtkColumnViewColumn {
	return C.gtk_column_view_sorter_get_primary_sort_column(self)
}

func gtkColumnViewSorterGetPrimarySortOrder(self *gtkColumnViewSorter) gtkSortType {
	return C.gtk_column_view_sorter_get_primary_sort_order(self)
}

func gtkCustomSorterNew(sort_func gCompareDataFunc, user_data unsafe.Pointer, user_destroy gDestroyNotify) *gtkCustomSorter {
	return C.gtk_custom_sorter_new(sort_func, C.gpointer(user_data), user_destroy)
}

func gtkDragSourceNew() *gtkDragSource { return C.gtk_drag_source_new() }

func gtkDragSourceSetActions(source *gtkDragSource, actions gdkDragAction) {
	C.gtk_drag_source_set_actions(source, actions)
}

func gtkDragSourceSetIcon(source *gtkDragSource, paintable *gdkPaintable, hot_x cInt, hot_y cInt) {
	C.gtk_drag_source_set_icon(source, paintable, hot_x, hot_y)
}

func gtkDrawingAreaGetType() gType { return C.gtk_drawing_area_get_type() }

func gtkDrawingAreaSetContentHeight(self *gtkDrawingArea, height cInt) {
	C.gtk_drawing_area_set_content_height(self, height)
}

func gtkDrawingAreaSetContentWidth(self *gtkDrawingArea, width cInt) {
	C.gtk_drawing_area_set_content_width(self, width)
}

func gtkDrawingAreaSetDrawFunc(self *gtkDrawingArea, draw_func gtkDrawingAreaDrawFunc, user_data unsafe.Pointer, destroy gDestroyNotify) {
	C.gtk_drawing_area_set_draw_func(self, draw_func, (C.gpointer)(unsafe.Pointer(user_data)), destroy)
}

func gtkDropDownGetModel(self *gtkDropDown) *gListModel { return C.gtk_drop_down_get_model(self) }

func gtkDropDownGetSelected(self *gtkDropDown) cUint { return C.gtk_drop_down_get_selected(self) }

func gtkDropDownNew(model *gListModel, expression *gtkExpression) *gtkWidget {
	return C.gtk_drop_down_new(model, expression)
}

func gtkDropDownNewFromStrings(strings **cChar) *gtkWidget {
	return C.gtk_drop_down_new_from_strings(strings)
}

func gtkDropDownSetListFactory(self *gtkDropDown, factory *gtkListItemFactory) {
	C.gtk_drop_down_set_list_factory(self, factory)
}

func gtkDropDownSetModel(self *gtkDropDown, model *gListModel) {
	C.gtk_drop_down_set_model(self, model)
}

func gtkDropDownSetSelected(self *gtkDropDown, position cUint) {
	C.gtk_drop_down_set_selected(self, position)
}

func gtkDropTargetAsyncNew(formats *gdkContentFormats, actions gdkDragAction) *gtkDropTargetAsync {
	return C.gtk_drop_target_async_new(formats, actions)
}

func gtkDropTargetNew(arg0 gType, actions gdkDragAction) *gtkDropTarget {
	return C.gtk_drop_target_new(arg0, actions)
}

func gtkEditableGetPosition(editable *gtkEditable) cInt { return C.gtk_editable_get_position(editable) }

func gtkEditableGetSelectionBounds(editable *gtkEditable, start_pos *cInt, end_pos *cInt) gBoolean {
	return C.gtk_editable_get_selection_bounds(editable, start_pos, end_pos)
}

func gtkEditableGetText(editable *gtkEditable) *cChar { return C.gtk_editable_get_text(editable) }

func gtkEditableGetType() gType { return C.gtk_editable_get_type() }

func gtkEditableSelectRegion(editable *gtkEditable, start_pos cInt, end_pos cInt) {
	C.gtk_editable_select_region(editable, start_pos, end_pos)
}

func gtkEditableSetAlignment(editable *gtkEditable, xalign cFloat) {
	C.gtk_editable_set_alignment(editable, xalign)
}

func gtkEditableSetEditable(editable *gtkEditable, is_editable gBoolean) {
	C.gtk_editable_set_editable(editable, is_editable)
}

func gtkEditableSetText(editable *gtkEditable, text *cChar) { C.gtk_editable_set_text(editable, text) }

func gtkEntryGetPlaceholderText(entry *gtkEntry) *cChar {
	return C.gtk_entry_get_placeholder_text(entry)
}

func gtkEntryGetType() gType { return C.gtk_entry_get_type() }

func gtkEntryNew() *gtkWidget { return C.gtk_entry_new() }

func gtkEntrySetInputHints(entry *gtkEntry, hints gtkInputHints) {
	C.gtk_entry_set_input_hints(entry, hints)
}

func gtkEntrySetInputPurpose(entry *gtkEntry, purpose gtkInputPurpose) {
	C.gtk_entry_set_input_purpose(entry, purpose)
}

func gtkEntrySetMaxLength(entry *gtkEntry, max cInt) { C.gtk_entry_set_max_length(entry, max) }

func gtkEntrySetPlaceholderText(entry *gtkEntry, text *cChar) {
	C.gtk_entry_set_placeholder_text(entry, text)
}

func gtkEventControllerFocusNew() *gtkEventController { return C.gtk_event_controller_focus_new() }

func gtkEventControllerGetCurrentEventState(controller *gtkEventController) gdkModifierType {
	return C.gtk_event_controller_get_current_event_state(controller)
}

func gtkEventControllerKeyNew() *gtkEventController { return C.gtk_event_controller_key_new() }

func gtkEventControllerLegacyNew() *gtkEventController { return C.gtk_event_controller_legacy_new() }

func gtkEventControllerMotionNew() *gtkEventController { return C.gtk_event_controller_motion_new() }

func gtkEventControllerScrollNew(flags gtkEventControllerScrollFlags) *gtkEventController {
	return C.gtk_event_controller_scroll_new(flags)
}

func gtkEventControllerSetPropagationPhase(controller *gtkEventController, phase gtkPropagationPhase) {
	C.gtk_event_controller_set_propagation_phase(controller, phase)
}

func gtkExpanderNew(label *cChar) *gtkWidget { return C.gtk_expander_new(label) }

func gtkExpanderSetChild(expander *gtkExpander, child *gtkWidget) {
	C.gtk_expander_set_child(expander, child)
}

func gtkFileDialogNew() *gtkFileDialog { return C.gtk_file_dialog_new() }

func gtkFileDialogOpen(dialog *gtkFileDialog, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_file_dialog_open(dialog, parent, cancellable, callback, C.gpointer(data))
}

func gtkFileDialogOpenFinish(dialog *gtkFileDialog, result *gAsyncResult, err **gError) *gFile {
	return C.gtk_file_dialog_open_finish(dialog, result, err)
}

func gtkFileDialogOpenMultiple(dialog *gtkFileDialog, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_file_dialog_open_multiple(dialog, parent, cancellable, callback, C.gpointer(data))
}

func gtkFileDialogOpenMultipleFinish(dialog *gtkFileDialog, result *gAsyncResult, err **gError) *gListModel {
	return C.gtk_file_dialog_open_multiple_finish(dialog, result, err)
}

func gtkFileDialogSave(dialog *gtkFileDialog, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_file_dialog_save(dialog, parent, cancellable, callback, C.gpointer(data))
}

func gtkFileDialogSaveFinish(dialog *gtkFileDialog, result *gAsyncResult, err **gError) *gFile {
	return C.gtk_file_dialog_save_finish(dialog, result, err)
}

func gtkFileDialogSelectFolder(dialog *gtkFileDialog, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_file_dialog_select_folder(dialog, parent, cancellable, callback, C.gpointer(data))
}

func gtkFileDialogSelectFolderFinish(dialog *gtkFileDialog, result *gAsyncResult, err **gError) *gFile {
	return C.gtk_file_dialog_select_folder_finish(dialog, result, err)
}

func gtkFileDialogSetDefaultFilter(dialog *gtkFileDialog, filter *gtkFileFilter) {
	C.gtk_file_dialog_set_default_filter(dialog, filter)
}

func gtkFileDialogSetFilters(dialog *gtkFileDialog, filters *gListModel) {
	C.gtk_file_dialog_set_filters(dialog, filters)
}

func gtkFileDialogSetInitialFolder(dialog *gtkFileDialog, folder *gFile) {
	C.gtk_file_dialog_set_initial_folder(dialog, folder)
}

func gtkFileDialogSetInitialName(dialog *gtkFileDialog, name *cChar) {
	C.gtk_file_dialog_set_initial_name(dialog, name)
}

func gtkFileDialogSetModal(dialog *gtkFileDialog, modal gBoolean) {
	C.gtk_file_dialog_set_modal(dialog, modal)
}

func gtkFileDialogSetTitle(dialog *gtkFileDialog, title *cChar) {
	C.gtk_file_dialog_set_title(dialog, title)
}

func gtkFileFilterAddSuffix(filter *gtkFileFilter, suffix *cChar) {
	C.gtk_file_filter_add_suffix(filter, suffix)
}

func gtkFileFilterGetType() gType { return C.gtk_file_filter_get_type() }

func gtkFileFilterNew() *gtkFileFilter { return C.gtk_file_filter_new() }

func gtkFileFilterSetName(filter *gtkFileFilter, name *cChar) {
	C.gtk_file_filter_set_name(filter, name)
}

func gtkFileLauncherLaunch(launcher *gtkFileLauncher, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_file_launcher_launch(launcher, parent, cancellable, callback, C.gpointer(data))
}

func gtkFileLauncherLaunchFinish(launcher *gtkFileLauncher, result *gAsyncResult, err **gError) gBoolean {
	return C.gtk_file_launcher_launch_finish(launcher, result, err)
}

func gtkFileLauncherNew(file *gFile) *gtkFileLauncher { return C.gtk_file_launcher_new(file) }

func gtkFileLauncherSetAlwaysAsk(launcher *gtkFileLauncher, ask gBoolean) {
	C.gtk_file_launcher_set_always_ask(launcher, ask)
}

func gtkFlattenListModelNew(model *gListModel) *gtkFlattenListModel {
	return C.gtk_flatten_list_model_new(model)
}

func gtkFlowBoxChildGetType() gType { return C.gtk_flow_box_child_get_type() }

func gtkFlowBoxChildSetChild(self *gtkFlowBoxChild, child *gtkWidget) {
	C.gtk_flow_box_child_set_child(self, child)
}

func gtkFontDialogButtonGetFontDesc(self *gtkFontDialogButton) *pangoFontDescription {
	return C.gtk_font_dialog_button_get_font_desc(self)
}

func gtkFontDialogButtonNew(dialog *gtkFontDialog) *gtkWidget {
	return C.gtk_font_dialog_button_new(dialog)
}

func gtkFontDialogButtonSetFontDesc(self *gtkFontDialogButton, font_desc *pangoFontDescription) {
	C.gtk_font_dialog_button_set_font_desc(self, font_desc)
}

func gtkFontDialogButtonSetLevel(self *gtkFontDialogButton, level gtkFontLevel) {
	C.gtk_font_dialog_button_set_level(self, level)
}

func gtkFontDialogNew() *gtkFontDialog { return C.gtk_font_dialog_new() }

func gtkFrameGetType() gType { return C.gtk_frame_get_type() }

func gtkFrameSetChild(frame *gtkFrame, child *gtkWidget) { C.gtk_frame_set_child(frame, child) }

func gtkFrameSetLabelWidget(frame *gtkFrame, label_widget *gtkWidget) {
	C.gtk_frame_set_label_widget(frame, label_widget)
}

func gtkGestureClickNew() *gtkGesture { return C.gtk_gesture_click_new() }

func gtkGestureDragNew() *gtkGesture { return C.gtk_gesture_drag_new() }

func gtkGestureSetState(gesture *gtkGesture, state gtkEventSequenceState) gBoolean {
	return C.gtk_gesture_set_state(gesture, state)
}

func gtkGestureSingleGetCurrentButton(gesture *gtkGestureSingle) cUint {
	return C.gtk_gesture_single_get_current_button(gesture)
}

func gtkGestureSingleSetButton(gesture *gtkGestureSingle, button cUint) {
	C.gtk_gesture_single_set_button(gesture, button)
}

func gtkGridGetType() gType { return C.gtk_grid_get_type() }

func gtkGridRemove(grid *gtkGrid, child *gtkWidget) { C.gtk_grid_remove(grid, child) }

func gtkGridViewNew(model *gtkSelectionModel, factory *gtkListItemFactory) *gtkWidget {
	return C.gtk_grid_view_new(model, factory)
}

func gtkGridViewSetMaxColumns(self *gtkGridView, max_columns cUint) {
	C.gtk_grid_view_set_max_columns(self, max_columns)
}

func gtkIMContextFilterKeypress(context *gtkIMContext, event *gdkEvent) gBoolean {
	return C.gtk_im_context_filter_keypress(context, event)
}

func gtkIMContextFocusIn(context *gtkIMContext) { C.gtk_im_context_focus_in(context) }

func gtkIMContextFocusOut(context *gtkIMContext) { C.gtk_im_context_focus_out(context) }

func gtkIMContextGetPreeditString(context *gtkIMContext, str **cChar, attrs **pangoAttrList, cursor_pos *cInt) {
	C.gtk_im_context_get_preedit_string(context, str, attrs, cursor_pos)
}

func gtkIMContextSetClientWidget(context *gtkIMContext, widget *gtkWidget) {
	C.gtk_im_context_set_client_widget(context, widget)
}

func gtkIMMulticontextNew() *gtkIMContext { return C.gtk_im_multicontext_new() }

func gtkIconPaintableGetIconName(self *gtkIconPaintable) *cChar {
	return C.gtk_icon_paintable_get_icon_name(self)
}

func gtkIconThemeGetForDisplay(display *gdkDisplay) *gtkIconTheme {
	return C.gtk_icon_theme_get_for_display(display)
}

func gtkIconThemeHasIcon(theme *gtkIconTheme, name *cChar) gBoolean {
	return C.gtk_icon_theme_has_icon(theme, name)
}

func gtkIconThemeLookupIcon(theme *gtkIconTheme, name *cChar, fallbacks **cChar, size, scale cInt, direction gtkTextDirection, flags gtkIconLookupFlags) *gtkIconPaintable {
	return C.gtk_icon_theme_lookup_icon(theme, name, fallbacks, size, scale, direction, flags)
}

func gtkImageGetType() gType { return C.gtk_image_get_type() }

func gtkImageNew() *gtkWidget { return C.gtk_image_new() }

func gtkImageNewFromIconName(name *cChar) *gtkWidget { return C.gtk_image_new_from_icon_name(name) }

func gtkImageNewFromPaintable(paintable *gdkPaintable) *gtkWidget {
	return C.gtk_image_new_from_paintable(paintable)
}

func gtkImageSetFromIconName(image *gtkImage, icon_name *cChar) {
	C.gtk_image_set_from_icon_name(image, icon_name)
}

func gtkImageSetFromPaintable(image *gtkImage, paintable *gdkPaintable) {
	C.gtk_image_set_from_paintable(image, paintable)
}

func gtkImageSetPixelSize(image *gtkImage, size cInt) { C.gtk_image_set_pixel_size(image, size) }

func gtkInitCheck() gBoolean { return C.gtk_init_check() }

func gtkKeyvalTriggerNew(key cUint, modifiers gdkModifierType) *gtkShortcutTrigger {
	return C.gtk_keyval_trigger_new(key, modifiers)
}

func gtkLabelGetText(label *gtkLabel) *cChar { return C.gtk_label_get_text(label) }

func gtkLabelGetType() gType { return C.gtk_label_get_type() }

func gtkLabelGetWrap(self *gtkLabel) gBoolean { return C.gtk_label_get_wrap(self) }

func gtkLabelNew(text *cChar) *gtkWidget { return C.gtk_label_new(text) }

func gtkLabelSetEllipsize(self *gtkLabel, mode pangoEllipsizeMode) {
	C.gtk_label_set_ellipsize(self, mode)
}

func gtkLabelSetJustify(label *gtkLabel, justify gtkJustification) {
	C.gtk_label_set_justify(label, justify)
}

func gtkLabelSetLines(self *gtkLabel, lines cInt) { C.gtk_label_set_lines(self, lines) }

func gtkLabelSetMarkup(self *gtkLabel, str *cChar) { C.gtk_label_set_markup(self, str) }

func gtkLabelSetMaxWidthChars(label *gtkLabel, width cInt) {
	C.gtk_label_set_max_width_chars(label, width)
}

func gtkLabelSetSelectable(label *gtkLabel, selectable gBoolean) {
	C.gtk_label_set_selectable(label, selectable)
}

func gtkLabelSetSingleLineMode(self *gtkLabel, single_line_mode gBoolean) {
	C.gtk_label_set_single_line_mode(self, single_line_mode)
}

func gtkLabelSetText(self *gtkLabel, str *cChar) { C.gtk_label_set_text(self, str) }

func gtkLabelSetWrap(label *gtkLabel, wrap gBoolean) { C.gtk_label_set_wrap(label, wrap) }

func gtkLabelSetWrapMode(self *gtkLabel, wrap_mode pangoWrapMode) {
	C.gtk_label_set_wrap_mode(self, wrap_mode)
}

func gtkLabelSetXalign(label *gtkLabel, align cFloat) { C.gtk_label_set_xalign(label, align) }

func gtkLevelBarGetType() gType { return C.gtk_level_bar_get_type() }

func gtkLevelBarSetValue(self *gtkLevelBar, value cDouble) { C.gtk_level_bar_set_value(self, value) }

func gtkLinkButtonNewWithLabel(uri *cChar, label *cChar) *gtkWidget {
	return C.gtk_link_button_new_with_label(uri, label)
}

func gtkLinkButtonSetURI(link_button *gtkLinkButton, uri *cChar) {
	C.gtk_link_button_set_uri(link_button, uri)
}

func gtkLinkButtonSetVisited(link_button *gtkLinkButton, visited gBoolean) {
	C.gtk_link_button_set_visited(link_button, visited)
}

func gtkListBoxAppend(box *gtkListBox, child *gtkWidget) { C.gtk_list_box_append(box, child) }

func gtkListBoxNew() *gtkWidget { return C.gtk_list_box_new() }

func gtkListBoxRemoveAll(box *gtkListBox) { C.gtk_list_box_remove_all(box) }

func gtkListBoxRowGetChild(row *gtkListBoxRow) *gtkWidget { return C.gtk_list_box_row_get_child(row) }

func gtkListItemGetChild(self *gtkListItem) *gtkWidget { return C.gtk_list_item_get_child(self) }

func gtkListItemGetItem(self *gtkListItem) unsafe.Pointer {
	return unsafe.Pointer(C.gtk_list_item_get_item(self))
}

func gtkListItemGetPosition(self *gtkListItem) cUint { return C.gtk_list_item_get_position(self) }

func gtkListItemSetActivatable(self *gtkListItem, activatable gBoolean) {
	C.gtk_list_item_set_activatable(self, activatable)
}

func gtkListItemSetChild(self *gtkListItem, child *gtkWidget) { C.gtk_list_item_set_child(self, child) }

func gtkListItemSetSelectable(self *gtkListItem, selectable gBoolean) {
	C.gtk_list_item_set_selectable(self, selectable)
}

func gtkListViewNew(model *gtkSelectionModel, factory *gtkListItemFactory) *gtkWidget {
	return C.gtk_list_view_new(model, factory)
}

func gtkListViewScrollTo(self *gtkListView, pos cUint, flags gtkListScrollFlags, scroll *gtkScrollInfo) {
	C.gtk_list_view_scroll_to(self, pos, flags, scroll)
}

func gtkListViewSetSingleClickActivate(self *gtkListView, single_click_activate gBoolean) {
	C.gtk_list_view_set_single_click_activate(self, single_click_activate)
}

func gtkMediaFileNewForFile(file *gFile) *gtkMediaStream { return C.gtk_media_file_new_for_file(file) }

func gtkMediaStreamGetDuration(self *gtkMediaStream) cInt64 {
	return C.gtk_media_stream_get_duration(self)
}

func gtkMediaStreamGetPlaying(self *gtkMediaStream) gBoolean {
	return C.gtk_media_stream_get_playing(self)
}

func gtkMediaStreamGetTimestamp(self *gtkMediaStream) cInt64 {
	return C.gtk_media_stream_get_timestamp(self)
}

func gtkMediaStreamIsSeekable(self *gtkMediaStream) gBoolean {
	return C.gtk_media_stream_is_seekable(self)
}

func gtkMediaStreamPause(self *gtkMediaStream) { C.gtk_media_stream_pause(self) }

func gtkMediaStreamPlay(self *gtkMediaStream) { C.gtk_media_stream_play(self) }

func gtkMediaStreamSeek(self *gtkMediaStream, timestamp cInt64) {
	C.gtk_media_stream_seek(self, timestamp)
}

func gtkMediaStreamSetMuted(self *gtkMediaStream, muted gBoolean) {
	C.gtk_media_stream_set_muted(self, muted)
}

func gtkMenuButtonNew() *gtkWidget { return C.gtk_menu_button_new() }

func gtkMenuButtonPopdown(menu_button *gtkMenuButton) { C.gtk_menu_button_popdown(menu_button) }

func gtkMenuButtonSetIconName(menu_button *gtkMenuButton, icon_name *cChar) {
	C.gtk_menu_button_set_icon_name(menu_button, icon_name)
}

func gtkMenuButtonSetLabel(menu_button *gtkMenuButton, label *cChar) {
	C.gtk_menu_button_set_label(menu_button, label)
}

func gtkMenuButtonSetPopover(menu_button *gtkMenuButton, popover *gtkWidget) {
	C.gtk_menu_button_set_popover(menu_button, popover)
}

func gtkMultiSelectionNew(model *gListModel) *gtkMultiSelection {
	return C.gtk_multi_selection_new(model)
}

func gtkNamedActionNew(name *cChar) *gtkShortcutAction { return C.gtk_named_action_new(name) }

func gtkNativeGetSurface(self *gtkNative) *gdkSurface { return C.gtk_native_get_surface(self) }

func gtkNativeGetSurfaceTransform(self *gtkNative, x *cDouble, y *cDouble) {
	C.gtk_native_get_surface_transform(self, x, y)
}

func gtkNoSelectionNew(model *gListModel) *gtkNoSelection { return C.gtk_no_selection_new(model) }

func gtkNotebookAppendPage(notebook *gtkNotebook, child *gtkWidget, tab_label *gtkWidget) cInt {
	return C.gtk_notebook_append_page(notebook, child, tab_label)
}

func gtkNotebookGetCurrentPage(notebook *gtkNotebook) cInt {
	return C.gtk_notebook_get_current_page(notebook)
}

func gtkNotebookGetNPages(notebook *gtkNotebook) cInt { return C.gtk_notebook_get_n_pages(notebook) }

func gtkNotebookGetNthPage(notebook *gtkNotebook, page_num cInt) *gtkWidget {
	return C.gtk_notebook_get_nth_page(notebook, page_num)
}

func gtkNotebookGetTabPos(notebook *gtkNotebook) gtkPositionType {
	return C.gtk_notebook_get_tab_pos(notebook)
}

func gtkNotebookNew() *gtkWidget { return C.gtk_notebook_new() }

func gtkNotebookPopupDisable(notebook *gtkNotebook) { C.gtk_notebook_popup_disable(notebook) }

func gtkNotebookPopupEnable(notebook *gtkNotebook) { C.gtk_notebook_popup_enable(notebook) }

func gtkNotebookSetCurrentPage(notebook *gtkNotebook, page_num cInt) {
	C.gtk_notebook_set_current_page(notebook, page_num)
}

func gtkNotebookSetScrollable(notebook *gtkNotebook, scrollable gBoolean) {
	C.gtk_notebook_set_scrollable(notebook, scrollable)
}

func gtkNotebookSetTabPos(notebook *gtkNotebook, pos gtkPositionType) {
	C.gtk_notebook_set_tab_pos(notebook, pos)
}

func gtkOrientableSetOrientation(orientable *gtkOrientable, orientation gtkOrientation) {
	C.gtk_orientable_set_orientation(orientable, orientation)
}

func gtkOverlayAddOverlay(overlay *gtkOverlay, widget *gtkWidget) {
	C.gtk_overlay_add_overlay(overlay, widget)
}

func gtkOverlayGetChild(overlay *gtkOverlay) *gtkWidget { return C.gtk_overlay_get_child(overlay) }

func gtkOverlayGetType() gType { return C.gtk_overlay_get_type() }

func gtkOverlayNew() *gtkWidget { return C.gtk_overlay_new() }

func gtkOverlayRemoveOverlay(overlay *gtkOverlay, widget *gtkWidget) {
	C.gtk_overlay_remove_overlay(overlay, widget)
}

func gtkOverlaySetChild(overlay *gtkOverlay, child *gtkWidget) {
	C.gtk_overlay_set_child(overlay, child)
}

func gtkPanedGetPosition(paned *gtkPaned) cInt { return C.gtk_paned_get_position(paned) }

func gtkPanedGetStartChild(paned *gtkPaned) *gtkWidget { return C.gtk_paned_get_start_child(paned) }

func gtkPanedGetType() gType { return C.gtk_paned_get_type() }

func gtkPanedNew(orientation gtkOrientation) *gtkWidget { return C.gtk_paned_new(orientation) }

func gtkPanedSetEndChild(paned *gtkPaned, child *gtkWidget) { C.gtk_paned_set_end_child(paned, child) }

func gtkPanedSetPosition(paned *gtkPaned, position cInt) { C.gtk_paned_set_position(paned, position) }

func gtkPanedSetResizeStartChild(paned *gtkPaned, resize gBoolean) {
	C.gtk_paned_set_resize_start_child(paned, resize)
}

func gtkPanedSetShrinkEndChild(paned *gtkPaned, resize gBoolean) {
	C.gtk_paned_set_shrink_end_child(paned, resize)
}

func gtkPanedSetShrinkStartChild(paned *gtkPaned, resize gBoolean) {
	C.gtk_paned_set_shrink_start_child(paned, resize)
}

func gtkPanedSetStartChild(paned *gtkPaned, child *gtkWidget) {
	C.gtk_paned_set_start_child(paned, child)
}

func gtkPasswordEntryGetType() gType { return C.gtk_password_entry_get_type() }

func gtkPasswordEntryNew() *gtkWidget { return C.gtk_password_entry_new() }

func gtkPasswordEntrySetShowPeekIcon(entry *gtkPasswordEntry, show_peek_icon gBoolean) {
	C.gtk_password_entry_set_show_peek_icon(entry, show_peek_icon)
}

func gtkPictureGetType() gType { return C.gtk_picture_get_type() }

func gtkPictureNew() *gtkWidget { return C.gtk_picture_new() }

func gtkPictureNewForPaintable(paintable *gdkPaintable) *gtkWidget {
	return C.gtk_picture_new_for_paintable(paintable)
}

func gtkPictureSetContentFit(self *gtkPicture, content_fit gtkContentFit) {
	C.gtk_picture_set_content_fit(self, content_fit)
}

func gtkPictureSetPaintable(self *gtkPicture, paintable *gdkPaintable) {
	C.gtk_picture_set_paintable(self, paintable)
}

func gtkPopoverGetType() gType { return C.gtk_popover_get_type() }

func gtkPopoverMenuBarNewFromModel(model *gMenuModel) *gtkWidget {
	return C.gtk_popover_menu_bar_new_from_model(model)
}

func gtkPopoverMenuNewFromModel(model *gMenuModel) *gtkWidget {
	return C.gtk_popover_menu_new_from_model(model)
}

func gtkPopoverNew() *gtkWidget { return C.gtk_popover_new() }

func gtkPopoverPopdown(popover *gtkPopover) { C.gtk_popover_popdown(popover) }

func gtkPopoverPopup(popover *gtkPopover) { C.gtk_popover_popup(popover) }

func gtkPopoverSetAutohide(popover *gtkPopover, autohide gBoolean) {
	C.gtk_popover_set_autohide(popover, autohide)
}

func gtkPopoverSetChild(popover *gtkPopover, child *gtkWidget) {
	C.gtk_popover_set_child(popover, child)
}

func gtkPopoverSetHasArrow(popover *gtkPopover, has_arrow gBoolean) {
	C.gtk_popover_set_has_arrow(popover, has_arrow)
}

func gtkPopoverSetPointingTo(popover *gtkPopover, rect *gdkRectangle) {
	C.gtk_popover_set_pointing_to(popover, rect)
}

func gtkPopoverSetPosition(popover *gtkPopover, position gtkPositionType) {
	C.gtk_popover_set_position(popover, position)
}

func gtkPrintContextGetCairoContext(context *gtkPrintContext) *cairoContext {
	return C.gtk_print_context_get_cairo_context(context)
}

func gtkPrintContextGetHeight(context *gtkPrintContext) cDouble {
	return C.gtk_print_context_get_height(context)
}

func gtkPrintContextGetWidth(context *gtkPrintContext) cDouble {
	return C.gtk_print_context_get_width(context)
}

func gtkPrintOperationCancel(operation *gtkPrintOperation) {
	C.gtk_print_operation_cancel(operation)
}

func gtkPrintOperationNew() *gtkPrintOperation {
	return C.gtk_print_operation_new()
}

func gtkPrintOperationRun(operation *gtkPrintOperation, action gtkPrintOperationAction, parent *gtkWindow, err **gError) gtkPrintOperationResult {
	return C.gtk_print_operation_run(operation, action, parent, err)
}

func gtkPrintOperationSetExportFilename(operation *gtkPrintOperation, filename *cChar) {
	C.gtk_print_operation_set_export_filename(operation, filename)
}

func gtkPrintOperationSetJobName(operation *gtkPrintOperation, name *cChar) {
	C.gtk_print_operation_set_job_name(operation, name)
}

func gtkPrintOperationSetNPages(operation *gtkPrintOperation, pages cInt) {
	C.gtk_print_operation_set_n_pages(operation, pages)
}

func gtkPrintOperationSetUnit(operation *gtkPrintOperation, unit gtkUnit) {
	C.gtk_print_operation_set_unit(operation, unit)
}

func gtkPrintOperationSetUseFullPage(operation *gtkPrintOperation, fullPage gBoolean) {
	C.gtk_print_operation_set_use_full_page(operation, fullPage)
}

func gtkProgressBarGetType() gType { return C.gtk_progress_bar_get_type() }

func gtkProgressBarNew() *gtkWidget { return C.gtk_progress_bar_new() }

func gtkProgressBarPulse(pbar *gtkProgressBar) { C.gtk_progress_bar_pulse(pbar) }

func gtkProgressBarSetFraction(pbar *gtkProgressBar, fraction cDouble) {
	C.gtk_progress_bar_set_fraction(pbar, fraction)
}

func gtkProgressBarSetInverted(pbar *gtkProgressBar, inverted gBoolean) {
	C.gtk_progress_bar_set_inverted(pbar, inverted)
}

func gtkProgressBarSetPulseStep(pbar *gtkProgressBar, fraction cDouble) {
	C.gtk_progress_bar_set_pulse_step(pbar, fraction)
}

func gtkProgressBarSetShowText(pbar *gtkProgressBar, show_text gBoolean) {
	C.gtk_progress_bar_set_show_text(pbar, show_text)
}

func gtkProgressBarSetText(pbar *gtkProgressBar, text *cChar) {
	C.gtk_progress_bar_set_text(pbar, text)
}

func gtkRangeGetType() gType { return C.gtk_range_get_type() }

func gtkRangeGetValue(arg0 *gtkRange) cDouble { return C.gtk_range_get_value(arg0) }

func gtkRangeSetInverted(arg0 *gtkRange, setting gBoolean) { C.gtk_range_set_inverted(arg0, setting) }

func gtkRangeSetValue(arg0 *gtkRange, value cDouble) { C.gtk_range_set_value(arg0, value) }

func gtkRecentInfoGetURI(info *gtkRecentInfo) *cChar {
	return C.gtk_recent_info_get_uri(info)
}

func gtkRecentInfoHasApplication(info *gtkRecentInfo, application *cChar) gBoolean {
	return C.gtk_recent_info_has_application(info, application)
}

func gtkRecentInfoUnref(info *gtkRecentInfo) {
	C.gtk_recent_info_unref(info)
}

func gtkRecentManagerAddItem(manager *gtkRecentManager, uri *cChar) gBoolean {
	return C.gtk_recent_manager_add_item(manager, uri)
}

func gtkRecentManagerGetDefault() *gtkRecentManager {
	return C.gtk_recent_manager_get_default()
}

func gtkRecentManagerGetItems(manager *gtkRecentManager) *gList {
	return C.gtk_recent_manager_get_items(manager)
}

func gtkRecentManagerRemoveItem(manager *gtkRecentManager, uri *cChar, err **gError) gBoolean {
	return C.gtk_recent_manager_remove_item(manager, uri, err)
}

func gtkRevealerGetType() gType { return C.gtk_revealer_get_type() }

func gtkRevealerNew() *gtkWidget { return C.gtk_revealer_new() }

func gtkRevealerSetChild(revealer *gtkRevealer, child *gtkWidget) {
	C.gtk_revealer_set_child(revealer, child)
}

func gtkRevealerSetRevealChild(revealer *gtkRevealer, reveal_child gBoolean) {
	C.gtk_revealer_set_reveal_child(revealer, reveal_child)
}

func gtkRevealerSetTransitionDuration(revealer *gtkRevealer, duration cUint) {
	C.gtk_revealer_set_transition_duration(revealer, duration)
}

func gtkRevealerSetTransitionType(revealer *gtkRevealer, transition gtkRevealerTransitionType) {
	C.gtk_revealer_set_transition_type(revealer, transition)
}

func gtkRootGetFocus(self *gtkRoot) *gtkWidget { return C.gtk_root_get_focus(self) }

func gtkScaleAddMark(scale *gtkScale, value cDouble, position gtkPositionType, markup *cChar) {
	C.gtk_scale_add_mark(scale, value, position, markup)
}

func gtkScaleClearMarks(scale *gtkScale) { C.gtk_scale_clear_marks(scale) }

func gtkScaleNew(orientation gtkOrientation, adjustment *gtkAdjustment) *gtkWidget {
	return C.gtk_scale_new(orientation, adjustment)
}

func gtkScaleSetDrawValue(scale *gtkScale, draw_value gBoolean) {
	C.gtk_scale_set_draw_value(scale, draw_value)
}

func gtkScrolledWindowGetChild(scrolled_window *gtkScrolledWindow) *gtkWidget {
	return C.gtk_scrolled_window_get_child(scrolled_window)
}

func gtkScrolledWindowGetHadjustment(scrolled_window *gtkScrolledWindow) *gtkAdjustment {
	return C.gtk_scrolled_window_get_hadjustment(scrolled_window)
}

func gtkScrolledWindowGetType() gType { return C.gtk_scrolled_window_get_type() }

func gtkScrolledWindowGetVadjustment(scrolled_window *gtkScrolledWindow) *gtkAdjustment {
	return C.gtk_scrolled_window_get_vadjustment(scrolled_window)
}

func gtkScrolledWindowNew() *gtkWidget { return C.gtk_scrolled_window_new() }

func gtkScrolledWindowSetChild(scrolled_window *gtkScrolledWindow, child *gtkWidget) {
	C.gtk_scrolled_window_set_child(scrolled_window, child)
}

func gtkScrolledWindowSetHasFrame(scrolled_window *gtkScrolledWindow, has_frame gBoolean) {
	C.gtk_scrolled_window_set_has_frame(scrolled_window, has_frame)
}

func gtkScrolledWindowSetKineticScrolling(scrolled_window *gtkScrolledWindow, kinetic_scrolling gBoolean) {
	C.gtk_scrolled_window_set_kinetic_scrolling(scrolled_window, kinetic_scrolling)
}

func gtkScrolledWindowSetMaxContentHeight(scrolled_window *gtkScrolledWindow, height cInt) {
	C.gtk_scrolled_window_set_max_content_height(scrolled_window, height)
}

func gtkScrolledWindowSetOverlayScrolling(scrolled_window *gtkScrolledWindow, overlay_scrolling gBoolean) {
	C.gtk_scrolled_window_set_overlay_scrolling(scrolled_window, overlay_scrolling)
}

func gtkScrolledWindowSetPolicy(scrolled_window *gtkScrolledWindow, hscrollbar_policy gtkPolicyType, vscrollbar_policy gtkPolicyType) {
	C.gtk_scrolled_window_set_policy(scrolled_window, hscrollbar_policy, vscrollbar_policy)
}

func gtkScrolledWindowSetPropagateNaturalHeight(scrolled_window *gtkScrolledWindow, propagate gBoolean) {
	C.gtk_scrolled_window_set_propagate_natural_height(scrolled_window, propagate)
}

func gtkSearchEntryGetPlaceholderText(entry *gtkSearchEntry) *cChar {
	return C.gtk_search_entry_get_placeholder_text(entry)
}

func gtkSearchEntryGetType() gType { return C.gtk_search_entry_get_type() }

func gtkSearchEntryNew() *gtkWidget { return C.gtk_search_entry_new() }

func gtkSearchEntrySetPlaceholderText(entry *gtkSearchEntry, text *cChar) {
	C.gtk_search_entry_set_placeholder_text(entry, text)
}

func gtkSelectionModelIsSelected(model *gtkSelectionModel, position cUint) gBoolean {
	return C.gtk_selection_model_is_selected(model, position)
}

func gtkSelectionModelSetSelection(model *gtkSelectionModel, selected *gtkBitset, mask *gtkBitset) gBoolean {
	return C.gtk_selection_model_set_selection(model, selected, mask)
}

func gtkSeparatorNew(orientation gtkOrientation) *gtkWidget { return C.gtk_separator_new(orientation) }

func gtkSettingsGetDefault() *gtkSettings { return C.gtk_settings_get_default() }

func gtkShortcutControllerAddShortcut(controller *gtkShortcutController, shortcut *gtkShortcut) {
	C.gtk_shortcut_controller_add_shortcut(controller, shortcut)
}

func gtkShortcutControllerNew() *gtkEventController { return C.gtk_shortcut_controller_new() }

func gtkShortcutControllerNewForModel(model *gListModel) *gtkEventController {
	return C.gtk_shortcut_controller_new_for_model(model)
}

func gtkShortcutControllerSetScope(controller *gtkShortcutController, scope gtkShortcutScope) {
	C.gtk_shortcut_controller_set_scope(controller, scope)
}

func gtkShortcutGetType() gType { return C.gtk_shortcut_get_type() }

func gtkShortcutNew(trigger *gtkShortcutTrigger, action *gtkShortcutAction) *gtkShortcut {
	return C.gtk_shortcut_new(trigger, action)
}

func gtkShortcutTriggerParseString(string *cChar) *gtkShortcutTrigger {
	return C.gtk_shortcut_trigger_parse_string(string)
}

func gtkSignalListItemFactoryNew() *gtkListItemFactory { return C.gtk_signal_list_item_factory_new() }

func gtkSingleSelectionNew(model *gListModel) *gtkSingleSelection {
	return C.gtk_single_selection_new(model)
}

func gtkSingleSelectionSetAutoselect(self *gtkSingleSelection, autoselect gBoolean) {
	C.gtk_single_selection_set_autoselect(self, autoselect)
}

func gtkSingleSelectionSetCanUnselect(self *gtkSingleSelection, can_unselect gBoolean) {
	C.gtk_single_selection_set_can_unselect(self, can_unselect)
}

func gtkSingleSelectionSetSelected(self *gtkSingleSelection, position cUint) {
	C.gtk_single_selection_set_selected(self, position)
}

func gtkSnapshotAppendScaledTexture(snapshot *gtkSnapshot, texture *gdkTexture, filter gskScalingFilter, bounds *grapheneRect) {
	C.gtk_snapshot_append_scaled_texture(snapshot, texture, filter, bounds)
}

func gtkSnapshotPop(snapshot *gtkSnapshot) { C.gtk_snapshot_pop(snapshot) }

func gtkSnapshotPushClip(snapshot *gtkSnapshot, bounds *grapheneRect) {
	C.gtk_snapshot_push_clip(snapshot, bounds)
}

func gtkSpinButtonGetValue(spin_button *gtkSpinButton) cDouble {
	return C.gtk_spin_button_get_value(spin_button)
}

func gtkSpinButtonGetValueAsInt(spin_button *gtkSpinButton) cInt {
	return C.gtk_spin_button_get_value_as_int(spin_button)
}

func gtkSpinButtonNewWithRange(min cDouble, max cDouble, step cDouble) *gtkWidget {
	return C.gtk_spin_button_new_with_range(min, max, step)
}

func gtkSpinButtonSetDigits(spin_button *gtkSpinButton, digits cUint) {
	C.gtk_spin_button_set_digits(spin_button, digits)
}

func gtkSpinButtonSetNumeric(spin_button *gtkSpinButton, numeric gBoolean) {
	C.gtk_spin_button_set_numeric(spin_button, numeric)
}

func gtkSpinButtonSetValue(spin_button *gtkSpinButton, value cDouble) {
	C.gtk_spin_button_set_value(spin_button, value)
}

func gtkSpinButtonSetWrap(spin_button *gtkSpinButton, wrap gBoolean) {
	C.gtk_spin_button_set_wrap(spin_button, wrap)
}

func gtkSpinnerNew() *gtkWidget { return C.gtk_spinner_new() }

func gtkSpinnerSetSpinning(spinner *gtkSpinner, spinning gBoolean) {
	C.gtk_spinner_set_spinning(spinner, spinning)
}

func gtkStackGetType() gType { return C.gtk_stack_get_type() }

func gtkStackRemove(stack *gtkStack, child *gtkWidget) { C.gtk_stack_remove(stack, child) }

func gtkStringListAppend(self *gtkStringList, string *cChar) { C.gtk_string_list_append(self, string) }

func gtkStringListNew(strings **cChar) *gtkStringList { return C.gtk_string_list_new(strings) }

func gtkStringObjectGetString(self *gtkStringObject) *cChar {
	return C.gtk_string_object_get_string(self)
}

func gtkStyleContextAddProviderForDisplay(display *gdkDisplay, provider *gtkStyleProvider, priority cUint) {
	C.gtk_style_context_add_provider_for_display(display, provider, priority)
}

func gtkSvgNewFromBytes(bytes *gBytes) *gdkPaintable { return C.gkCallSVGNew(optionalGTK.svg, bytes) }

func gtkSwitchGetActive(self *gtkSwitch) gBoolean { return C.gtk_switch_get_active(self) }

func gtkSwitchGetType() gType { return C.gtk_switch_get_type() }

func gtkSwitchNew() *gtkWidget { return C.gtk_switch_new() }

func gtkSwitchSetActive(self *gtkSwitch, is_active gBoolean) {
	C.gtk_switch_set_active(self, is_active)
}

func gtkTextBufferApplyTag(buffer *gtkTextBuffer, tag *gtkTextTag, start *gtkTextIter, end *gtkTextIter) {
	C.gtk_text_buffer_apply_tag(buffer, tag, start, end)
}

func gtkTextBufferGetBounds(buffer *gtkTextBuffer, start *gtkTextIter, end *gtkTextIter) {
	C.gtk_text_buffer_get_bounds(buffer, start, end)
}

func gtkTextBufferGetCharCount(buffer *gtkTextBuffer) cInt {
	return C.gtk_text_buffer_get_char_count(buffer)
}

func gtkTextBufferGetEndIter(buffer *gtkTextBuffer, iter *gtkTextIter) {
	C.gtk_text_buffer_get_end_iter(buffer, iter)
}

func gtkTextBufferGetIterAtOffset(buffer *gtkTextBuffer, iter *gtkTextIter, char_offset cInt) {
	C.gtk_text_buffer_get_iter_at_offset(buffer, iter, char_offset)
}

func gtkTextBufferGetSelectionBounds(buffer *gtkTextBuffer, start *gtkTextIter, end *gtkTextIter) gBoolean {
	return C.gtk_text_buffer_get_selection_bounds(buffer, start, end)
}

func gtkTextBufferGetTagTable(buffer *gtkTextBuffer) *gtkTextTagTable {
	return C.gtk_text_buffer_get_tag_table(buffer)
}

func gtkTextBufferGetText(buffer *gtkTextBuffer, start *gtkTextIter, end *gtkTextIter, include_hidden_chars gBoolean) *cChar {
	return C.gtk_text_buffer_get_text(buffer, start, end, include_hidden_chars)
}

func gtkTextBufferInsert(buffer *gtkTextBuffer, iter *gtkTextIter, text *cChar, len cInt) {
	C.gtk_text_buffer_insert(buffer, iter, text, len)
}

func gtkTextBufferRemoveAllTags(buffer *gtkTextBuffer, start *gtkTextIter, end *gtkTextIter) {
	C.gtk_text_buffer_remove_all_tags(buffer, start, end)
}

func gtkTextBufferSelectRange(buffer *gtkTextBuffer, ins *gtkTextIter, bound *gtkTextIter) {
	C.gtk_text_buffer_select_range(buffer, ins, bound)
}

func gtkTextBufferSetText(buffer *gtkTextBuffer, text *cChar, len cInt) {
	C.gtk_text_buffer_set_text(buffer, text, len)
}

func gtkTextGetType() gType { return C.gtk_text_get_type() }

func gtkTextGetVisibility(self *gtkText) gBoolean { return C.gtk_text_get_visibility(self) }

func gtkTextIterEqual(lhs *gtkTextIter, rhs *gtkTextIter) gBoolean {
	return C.gtk_text_iter_equal(lhs, rhs)
}

func gtkTextIterForwardToTagToggle(iter *gtkTextIter, tag *gtkTextTag) gBoolean {
	return C.gtk_text_iter_forward_to_tag_toggle(iter, tag)
}

func gtkTextIterGetOffset(iter *gtkTextIter) cInt { return C.gtk_text_iter_get_offset(iter) }

func gtkTextIterGetTags(iter *gtkTextIter) *gSList { return C.gtk_text_iter_get_tags(iter) }

func gtkTextSetVisibility(self *gtkText, visible gBoolean) { C.gtk_text_set_visibility(self, visible) }

func gtkTextTagNew(name *cChar) *gtkTextTag { return C.gtk_text_tag_new(name) }

func gtkTextTagTableAdd(table *gtkTextTagTable, tag *gtkTextTag) gBoolean {
	return C.gtk_text_tag_table_add(table, tag)
}

func gtkTextViewGetBuffer(text_view *gtkTextView) *gtkTextBuffer {
	return C.gtk_text_view_get_buffer(text_view)
}

func gtkTextViewGetEditable(text_view *gtkTextView) gBoolean {
	return C.gtk_text_view_get_editable(text_view)
}

func gtkTextViewGetIterAtLocation(text_view *gtkTextView, iter *gtkTextIter, x cInt, y cInt) gBoolean {
	return C.gtk_text_view_get_iter_at_location(text_view, iter, x, y)
}

func gtkTextViewGetType() gType { return C.gtk_text_view_get_type() }

func gtkTextViewNew() *gtkWidget { return C.gtk_text_view_new() }

func gtkTextViewSetAcceptsTab(text_view *gtkTextView, accepts_tab gBoolean) {
	C.gtk_text_view_set_accepts_tab(text_view, accepts_tab)
}

func gtkTextViewSetBottomMargin(text_view *gtkTextView, bottom_margin cInt) {
	C.gtk_text_view_set_bottom_margin(text_view, bottom_margin)
}

func gtkTextViewSetCursorVisible(text_view *gtkTextView, setting gBoolean) {
	C.gtk_text_view_set_cursor_visible(text_view, setting)
}

func gtkTextViewSetEditable(text_view *gtkTextView, setting gBoolean) {
	C.gtk_text_view_set_editable(text_view, setting)
}

func gtkTextViewSetInputHints(text_view *gtkTextView, hints gtkInputHints) {
	C.gtk_text_view_set_input_hints(text_view, hints)
}

func gtkTextViewSetLeftMargin(text_view *gtkTextView, left_margin cInt) {
	C.gtk_text_view_set_left_margin(text_view, left_margin)
}

func gtkTextViewSetRightMargin(text_view *gtkTextView, right_margin cInt) {
	C.gtk_text_view_set_right_margin(text_view, right_margin)
}

func gtkTextViewSetTopMargin(text_view *gtkTextView, top_margin cInt) {
	C.gtk_text_view_set_top_margin(text_view, top_margin)
}

func gtkTextViewSetWrapMode(text_view *gtkTextView, wrap_mode gtkWrapMode) {
	C.gtk_text_view_set_wrap_mode(text_view, wrap_mode)
}

func gtkTextViewWindowToBufferCoords(text_view *gtkTextView, win gtkTextWindowType, window_x cInt, window_y cInt, buffer_x *cInt, buffer_y *cInt) {
	C.gtk_text_view_window_to_buffer_coords(text_view, win, window_x, window_y, buffer_x, buffer_y)
}

func gtkToggleButtonGetActive(toggle_button *gtkToggleButton) gBoolean {
	return C.gtk_toggle_button_get_active(toggle_button)
}

func gtkToggleButtonGetType() gType { return C.gtk_toggle_button_get_type() }

func gtkToggleButtonNew() *gtkWidget { return C.gtk_toggle_button_new() }

func gtkToggleButtonNewWithLabel(label *cChar) *gtkWidget {
	return C.gtk_toggle_button_new_with_label(label)
}

func gtkToggleButtonSetActive(toggle_button *gtkToggleButton, is_active gBoolean) {
	C.gtk_toggle_button_set_active(toggle_button, is_active)
}

func gtkToggleButtonSetGroup(toggle_button *gtkToggleButton, group *gtkToggleButton) {
	C.gtk_toggle_button_set_group(toggle_button, group)
}

func gtkTreeExpanderGetChild(self *gtkTreeExpander) *gtkWidget {
	return C.gtk_tree_expander_get_child(self)
}

func gtkTreeExpanderNew() *gtkWidget { return C.gtk_tree_expander_new() }

func gtkTreeExpanderSetChild(self *gtkTreeExpander, child *gtkWidget) {
	C.gtk_tree_expander_set_child(self, child)
}

func gtkTreeExpanderSetHideExpander(self *gtkTreeExpander, hide_expander gBoolean) {
	C.gtk_tree_expander_set_hide_expander(self, hide_expander)
}

func gtkTreeExpanderSetListRow(self *gtkTreeExpander, list_row *gtkTreeListRow) {
	C.gtk_tree_expander_set_list_row(self, list_row)
}

func gtkTreeListModelGetRow(self *gtkTreeListModel, position cUint) *gtkTreeListRow {
	return C.gtk_tree_list_model_get_row(self, position)
}

func gtkTreeListModelNew(root *gListModel, passthrough gBoolean, autoexpand gBoolean, create_func gtkTreeListModelCreateModelFunc, user_data unsafe.Pointer, user_destroy gDestroyNotify) *gtkTreeListModel {
	return C.gtk_tree_list_model_new(root, passthrough, autoexpand, create_func, C.gpointer(user_data), user_destroy)
}

func gtkTreeListRowGetExpanded(self *gtkTreeListRow) gBoolean {
	return C.gtk_tree_list_row_get_expanded(self)
}

func gtkTreeListRowGetItem(self *gtkTreeListRow) unsafe.Pointer {
	return unsafe.Pointer(C.gtk_tree_list_row_get_item(self))
}

func gtkTreeListRowSetExpanded(self *gtkTreeListRow, expanded gBoolean) {
	C.gtk_tree_list_row_set_expanded(self, expanded)
}

func gtkUriLauncherLaunch(launcher *gtkUriLauncher, parent *gtkWindow, cancellable *gCancellable, callback gAsyncReadyCallback, data unsafe.Pointer) {
	C.gtk_uri_launcher_launch(launcher, parent, cancellable, callback, C.gpointer(data))
}

func gtkUriLauncherLaunchFinish(launcher *gtkUriLauncher, result *gAsyncResult, err **gError) gBoolean {
	return C.gtk_uri_launcher_launch_finish(launcher, result, err)
}

func gtkUriLauncherNew(uri *cChar) *gtkUriLauncher { return C.gtk_uri_launcher_new(uri) }

func gtkVideoNew() *gtkWidget { return C.gtk_video_new() }

func gtkVideoSetMediaStream(self *gtkVideo, stream *gtkMediaStream) {
	C.gtk_video_set_media_stream(self, stream)
}

func gtkViewportGetType() gType { return C.gtk_viewport_get_type() }

func gtkViewportSetChild(viewport *gtkViewport, child *gtkWidget) {
	C.gtk_viewport_set_child(viewport, child)
}

func gtkWidgetActivate(widget *gtkWidget) gBoolean { return C.gtk_widget_activate(widget) }

func gtkWidgetAddCSSClass(widget *gtkWidget, name *cChar) { C.gtk_widget_add_css_class(widget, name) }

func gtkWidgetAddController(widget *gtkWidget, controller *gtkEventController) {
	C.gtk_widget_add_controller(widget, controller)
}

func gtkWidgetAddTickCallback(widget *gtkWidget, callback gtkTickCallback, user_data unsafe.Pointer, notify gDestroyNotify) cUint {
	return C.gtk_widget_add_tick_callback(widget, callback, C.gpointer(user_data), notify)
}

func gtkWidgetAllocate(widget *gtkWidget, width cInt, height cInt, baseline cInt, transform *gskTransform) {
	C.gtk_widget_allocate(widget, width, height, baseline, transform)
}

func gtkWidgetClassSetAccessibleRole(widget_class *gtkWidgetClass, accessible_role gtkAccessibleRole) {
	C.gtk_widget_class_set_accessible_role(widget_class, accessible_role)
}

func gtkWidgetComputeBounds(widget *gtkWidget, target *gtkWidget, out_bounds *grapheneRect) gBoolean {
	return C.gtk_widget_compute_bounds(widget, target, out_bounds)
}

func gtkWidgetComputePoint(widget *gtkWidget, target *gtkWidget, point *graphenePoint, out_point *graphenePoint) gBoolean {
	return C.gtk_widget_compute_point(widget, target, point, out_point)
}

func gtkWidgetGetAllocation(widget *gtkWidget, allocation *gtkAllocation) {
	C.gtk_widget_get_allocation(widget, allocation)
}

func gtkWidgetGetCanFocus(widget *gtkWidget) gBoolean { return C.gtk_widget_get_can_focus(widget) }

func gtkWidgetGetChildVisible(widget *gtkWidget) gBoolean {
	return C.gtk_widget_get_child_visible(widget)
}

func gtkWidgetGetColor(widget *gtkWidget, color *gdkRGBA) { C.gtk_widget_get_color(widget, color) }

func gtkWidgetGetDefaultDirection() gtkTextDirection { return C.gtk_widget_get_default_direction() }

func gtkWidgetGetDirection(widget *gtkWidget) gtkTextDirection {
	return C.gtk_widget_get_direction(widget)
}

func gtkWidgetGetFirstChild(widget *gtkWidget) *gtkWidget {
	return C.gtk_widget_get_first_child(widget)
}

func gtkWidgetGetFocusable(widget *gtkWidget) gBoolean { return C.gtk_widget_get_focusable(widget) }

func gtkWidgetGetFrameClock(widget *gtkWidget) *gdkFrameClock {
	return C.gtk_widget_get_frame_clock(widget)
}

func gtkWidgetGetHeight(widget *gtkWidget) cInt { return C.gtk_widget_get_height(widget) }

func gtkWidgetGetLastChild(widget *gtkWidget) *gtkWidget { return C.gtk_widget_get_last_child(widget) }

func gtkWidgetGetMapped(widget *gtkWidget) gBoolean { return C.gtk_widget_get_mapped(widget) }

func gtkWidgetGetName(widget *gtkWidget) *cChar { return C.gtk_widget_get_name(widget) }

func gtkWidgetGetNative(widget *gtkWidget) *gtkNative { return C.gtk_widget_get_native(widget) }

func gtkWidgetGetNextSibling(widget *gtkWidget) *gtkWidget {
	return C.gtk_widget_get_next_sibling(widget)
}

func gtkWidgetGetParent(widget *gtkWidget) *gtkWidget { return C.gtk_widget_get_parent(widget) }

func gtkWidgetGetRoot(widget *gtkWidget) *gtkRoot { return C.gtk_widget_get_root(widget) }

func gtkWidgetGetScaleFactor(widget *gtkWidget) cInt { return C.gtk_widget_get_scale_factor(widget) }

func gtkWidgetGetType() gType { return C.gtk_widget_get_type() }

func gtkWidgetGetVisible(widget *gtkWidget) gBoolean {
	return C.gtk_widget_get_visible(widget)
}

func gtkWidgetGetWidth(widget *gtkWidget) cInt { return C.gtk_widget_get_width(widget) }

func gtkWidgetGrabFocus(widget *gtkWidget) gBoolean { return C.gtk_widget_grab_focus(widget) }

func gtkWidgetHasFocus(widget *gtkWidget) gBoolean { return C.gtk_widget_has_focus(widget) }

func gtkWidgetInsertActionGroup(widget *gtkWidget, name *cChar, group *gActionGroup) {
	C.gtk_widget_insert_action_group(widget, name, group)
}

func gtkWidgetInsertBefore(widget *gtkWidget, parent *gtkWidget, next_sibling *gtkWidget) {
	C.gtk_widget_insert_before(widget, parent, next_sibling)
}

func gtkWidgetIsAncestor(widget *gtkWidget, ancestor *gtkWidget) gBoolean {
	return C.gtk_widget_is_ancestor(widget, ancestor)
}

func gtkWidgetIsSensitive(widget *gtkWidget) gBoolean { return C.gtk_widget_is_sensitive(widget) }

func gtkWidgetMeasure(widget *gtkWidget, orientation gtkOrientation, for_size cInt, minimum *cInt, natural *cInt, minimum_baseline *cInt, natural_baseline *cInt) {
	C.gtk_widget_measure(widget, orientation, for_size, minimum, natural, minimum_baseline, natural_baseline)
}

func gtkWidgetPaintableNew(widget *gtkWidget) *gdkPaintable {
	return C.gtk_widget_paintable_new(widget)
}

func gtkWidgetQueueAllocate(widget *gtkWidget) { C.gtk_widget_queue_allocate(widget) }

func gtkWidgetQueueDraw(widget *gtkWidget) { C.gtk_widget_queue_draw(widget) }

func gtkWidgetQueueResize(widget *gtkWidget) { C.gtk_widget_queue_resize(widget) }

func gtkWidgetRealize(widget *gtkWidget) { C.gtk_widget_realize(widget) }

func gtkWidgetRemoveCSSClass(widget *gtkWidget, css_class *cChar) {
	C.gtk_widget_remove_css_class(widget, css_class)
}

func gtkWidgetRemoveController(widget *gtkWidget, controller *gtkEventController) {
	C.gtk_widget_remove_controller(widget, controller)
}

func gtkWidgetRemoveTickCallback(widget *gtkWidget, id cUint) {
	C.gtk_widget_remove_tick_callback(widget, id)
}

func gtkWidgetSetCanFocus(widget *gtkWidget, can_focus gBoolean) {
	C.gtk_widget_set_can_focus(widget, can_focus)
}

func gtkWidgetSetCanTarget(widget *gtkWidget, can_target gBoolean) {
	C.gtk_widget_set_can_target(widget, can_target)
}

func gtkWidgetSetChildVisible(widget *gtkWidget, child_visible gBoolean) {
	C.gtk_widget_set_child_visible(widget, child_visible)
}

func gtkWidgetSetCursorFromName(widget *gtkWidget, name *cChar) {
	C.gtk_widget_set_cursor_from_name(widget, name)
}

func gtkWidgetSetDirection(widget *gtkWidget, dir gtkTextDirection) {
	C.gtk_widget_set_direction(widget, dir)
}

func gtkWidgetSetFocusable(widget *gtkWidget, focusable gBoolean) {
	C.gtk_widget_set_focusable(widget, focusable)
}

func gtkWidgetSetHalign(widget *gtkWidget, align gtkAlign) { C.gtk_widget_set_halign(widget, align) }

func gtkWidgetSetHexpand(widget *gtkWidget, expand gBoolean) {
	C.gtk_widget_set_hexpand(widget, expand)
}

func gtkWidgetSetLayoutManager(widget *gtkWidget, layout_manager *gtkLayoutManager) {
	C.gtk_widget_set_layout_manager(widget, layout_manager)
}

func gtkWidgetSetMarginBottom(widget *gtkWidget, margin cInt) {
	C.gtk_widget_set_margin_bottom(widget, margin)
}

func gtkWidgetSetMarginEnd(widget *gtkWidget, margin cInt) {
	C.gtk_widget_set_margin_end(widget, margin)
}

func gtkWidgetSetMarginStart(widget *gtkWidget, margin cInt) {
	C.gtk_widget_set_margin_start(widget, margin)
}

func gtkWidgetSetMarginTop(widget *gtkWidget, margin cInt) {
	C.gtk_widget_set_margin_top(widget, margin)
}

func gtkWidgetSetOpacity(widget *gtkWidget, opacity cDouble) {
	C.gtk_widget_set_opacity(widget, opacity)
}

func gtkWidgetSetOverflow(widget *gtkWidget, overflow gtkOverflow) {
	C.gtk_widget_set_overflow(widget, overflow)
}

func gtkWidgetSetParent(widget, parent *gtkWidget) { C.gtk_widget_set_parent(widget, parent) }

func gtkWidgetSetSensitive(widget *gtkWidget, sensitive gBoolean) {
	C.gtk_widget_set_sensitive(widget, sensitive)
}

func gtkWidgetSetSizeRequest(widget *gtkWidget, width, height cInt) {
	C.gtk_widget_set_size_request(widget, width, height)
}

func gtkWidgetSetTooltipText(widget *gtkWidget, text *cChar) {
	C.gtk_widget_set_tooltip_text(widget, text)
}

func gtkWidgetSetValign(widget *gtkWidget, align gtkAlign) { C.gtk_widget_set_valign(widget, align) }

func gtkWidgetSetVexpand(widget *gtkWidget, expand gBoolean) {
	C.gtk_widget_set_vexpand(widget, expand)
}

func gtkWidgetSetVisible(widget *gtkWidget, visible gBoolean) {
	C.gtk_widget_set_visible(widget, visible)
}

func gtkWidgetShouldLayout(widget *gtkWidget) gBoolean { return C.gtk_widget_should_layout(widget) }

func gtkWidgetSnapshotChild(widget *gtkWidget, child *gtkWidget, snapshot *gtkSnapshot) {
	C.gtk_widget_snapshot_child(widget, child, snapshot)
}

func gtkWidgetUnparent(widget *gtkWidget) { C.gtk_widget_unparent(widget) }

func gtkWindowClose(window *gtkWindow) { C.gtk_window_close(window) }

func gtkWindowDestroy(window *gtkWindow) { C.gtk_window_destroy(window) }

func gtkWindowFullscreen(window *gtkWindow) { C.gtk_window_fullscreen(window) }

func gtkWindowGetChild(window *gtkWindow) *gtkWidget { return C.gtk_window_get_child(window) }

func gtkWindowGetTitle(window *gtkWindow) *cChar { return C.gtk_window_get_title(window) }

func gtkWindowGetType() gType { return C.gtk_window_get_type() }

func gtkWindowIsActive(window *gtkWindow) gBoolean { return C.gtk_window_is_active(window) }

func gtkWindowIsMaximized(window *gtkWindow) gBoolean { return C.gtk_window_is_maximized(window) }

func gtkWindowListToplevels() *gList { return C.gtk_window_list_toplevels() }

func gtkWindowMaximize(window *gtkWindow) { C.gtk_window_maximize(window) }

func gtkWindowMinimize(window *gtkWindow) { C.gtk_window_minimize(window) }

func gtkWindowNew() *gtkWidget { return C.gtk_window_new() }

func gtkWindowPresent(window *gtkWindow) {
	C.gtk_window_present(window)
}

func gtkWindowSetChild(window *gtkWindow, child *gtkWidget) { C.gtk_window_set_child(window, child) }

func gtkWindowSetDecorated(window *gtkWindow, setting gBoolean) {
	C.gtk_window_set_decorated(window, setting)
}

func gtkWindowSetDefaultSize(window *gtkWindow, width cInt, height cInt) {
	C.gtk_window_set_default_size(window, width, height)
}

func gtkWindowSetDefaultWidget(window *gtkWindow, widget *gtkWidget) {
	C.gtk_window_set_default_widget(window, widget)
}

func gtkWindowSetDeletable(window *gtkWindow, setting gBoolean) {
	C.gtk_window_set_deletable(window, setting)
}

func gtkWindowSetHideOnClose(window *gtkWindow, hide gBoolean) {
	C.gtk_window_set_hide_on_close(window, hide)
}

func gtkWindowSetModal(window *gtkWindow, modal gBoolean) { C.gtk_window_set_modal(window, modal) }

func gtkWindowSetResizable(window *gtkWindow, resizable gBoolean) {
	C.gtk_window_set_resizable(window, resizable)
}

func gtkWindowSetTitle(window *gtkWindow, title *cChar) { C.gtk_window_set_title(window, title) }

func gtkWindowSetTransientFor(window, parent *gtkWindow) {
	C.gtk_window_set_transient_for(window, parent)
}

func gtkWindowUnfullscreen(window *gtkWindow) { C.gtk_window_unfullscreen(window) }

func gtkWindowUnmaximize(window *gtkWindow) { C.gtk_window_unmaximize(window) }

func gtkWindowUnminimize(window *gtkWindow) { C.gtk_window_unminimize(window) }

func pangoCairoCreateLayout(context *cairoContext) *pangoLayout {
	return C.pango_cairo_create_layout(context)
}

func pangoCairoFontMapGetDefault() *pangoFontMap {
	return C.pango_cairo_font_map_get_default()
}

func pangoCairoShowLayout(context *cairoContext, layout *pangoLayout) {
	C.pango_cairo_show_layout(context, layout)
}

func pangoFontDescriptionFree(description *pangoFontDescription) {
	C.pango_font_description_free(description)
}

func pangoFontDescriptionFromString(str *cChar) *pangoFontDescription {
	return C.pango_font_description_from_string(str)
}

func pangoFontDescriptionGetFamily(desc *pangoFontDescription) *cChar {
	return C.pango_font_description_get_family(desc)
}

func pangoFontDescriptionGetSize(desc *pangoFontDescription) gint {
	return C.pango_font_description_get_size(desc)
}

func pangoFontDescriptionGetStyle(desc *pangoFontDescription) pangoStyle {
	return C.pango_font_description_get_style(desc)
}

func pangoFontDescriptionGetWeight(desc *pangoFontDescription) pangoWeight {
	return C.pango_font_description_get_weight(desc)
}

func pangoFontDescriptionNew() *pangoFontDescription {
	return C.pango_font_description_new()
}

func pangoFontDescriptionSetFamily(description *pangoFontDescription, family *cChar) {
	C.pango_font_description_set_family(description, family)
}

func pangoFontDescriptionSetSize(description *pangoFontDescription, size cInt) {
	C.pango_font_description_set_size(description, size)
}

func pangoFontDescriptionSetStyle(description *pangoFontDescription, style pangoStyle) {
	C.pango_font_description_set_style(description, style)
}

func pangoFontDescriptionSetWeight(description *pangoFontDescription, weight pangoWeight) {
	C.pango_font_description_set_weight(description, weight)
}

func pangoFontFamilyGetName(family *pangoFontFamily) *cChar {
	return C.pango_font_family_get_name(family)
}

func pangoFontMapListFamilies(fontMap *pangoFontMap, families ***pangoFontFamily, count *cInt) {
	C.pango_font_map_list_families(fontMap, families, count)
}

func pangoLayoutSetFontDescription(layout *pangoLayout, description *pangoFontDescription) {
	C.pango_layout_set_font_description(layout, description)
}

func pangoLayoutSetText(layout *pangoLayout, text *cChar, length cInt) {
	C.pango_layout_set_text(layout, text, length)
}

func webkitSettingsSetEnableJavaScript(settings *webkitSettings, enabled gBoolean) {
	C.gkCallWebJavaScript(webKitSymbols.scripts, settings, enabled)
}

func webkitWebViewGetSettings(view *webkitWebView) *webkitSettings {
	return C.gkCallWebSettings(webKitSymbols.settings, view)
}

func webkitWebViewGetURI(view *webkitWebView) *cChar {
	return C.gkCallWebGetURI(webKitSymbols.getURI, view)
}

func webkitWebViewLoadHTML(view *webkitWebView, html, base *cChar) {
	C.gkCallWebHTML(webKitSymbols.loadHTML, view, html, base)
}

func webkitWebViewLoadURI(view *webkitWebView, uri *cChar) {
	C.gkCallWebURI(webKitSymbols.loadURI, view, uri)
}

func webkitWebViewNew() *webkitWebView { return C.gkCallWebNew(webKitSymbols.newView) }

func webkitWebViewStopLoading(view *webkitWebView) { C.gkCallWebStop(webKitSymbols.stop, view) }

// Native callback addresses.
func gtkActionCallback() gCallback { return C.GCallback(C.goGTKAction) }

func gtkAsyncReadyCallback() gAsyncReadyCallback { return C.GAsyncReadyCallback(C.goGTKAsyncReady) }

func gtkBusAppearedCallback() gBusNameAppearedCallback {
	return C.GBusNameAppearedCallback(C.goGTKBusAppeared)
}

func gtkBusVanishedCallback() gBusNameVanishedCallback {
	return C.GBusNameVanishedCallback(C.goGTKBusVanished)
}

func gtkCallbackFree() gClosureNotify { return C.GClosureNotify(C.goGTKCallbackFree) }

func gtkCloseRequestCallback() gCallback { return C.GCallback(C.goGTKCloseRequest) }

func gtkDBusMethodCallback() gDBusInterfaceMethodCallFunc {
	return C.GDBusInterfaceMethodCallFunc(C.goGTKDBusMethod)
}

func gtkDBusPropertyCallback() gDBusInterfaceGetPropertyFunc {
	return C.GDBusInterfaceGetPropertyFunc(C.goGTKDBusProperty)
}

func gtkDBusSignalCallback() gDBusSignalCallback { return C.GDBusSignalCallback(C.goGTKDBusSignal) }

func gtkDestroyCallback() gDestroyNotify { return C.GDestroyNotify(C.goGTKDestroy) }

func gtkDragPrepareCallback() gCallback { return C.GCallback(C.goGTKDragPrepare) }

func gtkDrawCallback() gtkDrawingAreaDrawFunc { return C.GtkDrawingAreaDrawFunc(C.goGTKDraw) }

func gtkDropAcceptCallback() gCallback { return C.GCallback(C.goGTKDropAccept) }

func gtkDropEnterCallback() gCallback { return C.GCallback(C.goGTKDropEnter) }

func gtkDropMotionCallback() gCallback { return C.GCallback(C.goGTKDropMotion) }

func gtkDropPerformCallback() gCallback { return C.GCallback(C.goGTKDropPerform) }

func gtkEqualSortCallback() gCompareDataFunc { return C.GCompareDataFunc(C.goGTKEqualSort) }

func gtkImageClassCallback() gClassInitFunc { return C.GClassInitFunc(C.goGTKImageClass) }

func gtkImageMeasureAddress() *[0]byte { return (*[0]byte)(C.goGTKImageMeasure) }

func gtkImageSnapshotAddress() *[0]byte { return (*[0]byte)(C.goGTKImageSnapshot) }

func gtkIntSignalCallback() gCallback { return C.GCallback(C.goGTKIntSignal) }

func gtkKeyReleasedCallback() gCallback { return C.GCallback(C.goGTKKeyReleased) }

func gtkKeySignalCallback() gCallback { return C.GCallback(C.goGTKKeySignal) }

func gtkLayoutAllocateAddress() *[0]byte { return (*[0]byte)(C.goGTKLayoutAllocate) }

func gtkLayoutClassCallback() gClassInitFunc { return C.GClassInitFunc(C.goGTKLayoutClass) }

func gtkLayoutMeasureAddress() *[0]byte { return (*[0]byte)(C.goGTKLayoutMeasure) }

func gtkLayoutSnapshotAddress() *[0]byte { return (*[0]byte)(C.goGTKLayoutSnapshot) }

func gtkLegacyEventCallback() gCallback { return C.GCallback(C.goGTKLegacyEvent) }

func gtkListItemCallback() gCallback { return C.GCallback(C.goGTKListItem) }

func gtkNotifyCallback() gCallback { return C.GCallback(C.goGTKNotify) }

func gtkObjectSignalCallback() gCallback { return C.GCallback(C.goGTKObjectSignal) }

func gtkOverlayPositionCallback() gCallback { return C.GCallback(C.goGTKOverlayPosition) }

func gtkPressedCallback() gCallback { return C.GCallback(C.goGTKPressed) }

func gtkPrintBeginCallback() gCallback {
	return C.GCallback(C.goGTKPrintBegin)
}

func gtkPrintDrawCallback() gCallback {
	return C.GCallback(C.goGTKPrintDraw)
}

func gtkRangeValueCallback() gCallback { return C.GCallback(C.goGTKRangeValue) }

func gtkScrollSignalCallback() gCallback { return C.GCallback(C.goGTKScrollSignal) }

func gtkSelectionSignalCallback() gCallback { return C.GCallback(C.goGTKSelectionSignal) }

func gtkSignalCallback() gCallback { return C.GCallback(C.goGTKSignal) }

func gtkSourceCallback() gSourceFunc { return C.GSourceFunc(C.goGTKSource) }

func gtkStringSignalCallback() gCallback { return C.GCallback(C.goGTKStringSignal) }

func gtkTabSwitchCallback() gCallback { return C.GCallback(C.goGTKTabSwitch) }

func gtkTextInsertCallback() gCallback { return C.GCallback(C.goGTKTextInsert) }

func gtkTickCallbackAddress() gtkTickCallback { return C.GtkTickCallback(C.goGTKTick) }

func gtkTreeChildrenCallback() gtkTreeListModelCreateModelFunc {
	return C.GtkTreeListModelCreateModelFunc(C.goGTKTreeChildren)
}

func gtkUIntSignalCallback() gCallback { return C.GCallback(C.goGTKUIntSignal) }

func gtkValueDropCallback() gCallback { return C.GCallback(C.goGTKValueDrop) }

func gtkXYSignalCallback() gCallback { return C.GCallback(C.goGTKXYSignal) }

// Callback ABI adapters dispatch to Go-owned state and behavior.
//
//export goGTKAction
func goGTKAction(instance unsafe.Pointer, parameter *C.GVariant, data unsafe.Pointer) {
	nativeCallback(data).(func(*gVariant))(parameter)
}

//export goGTKAsyncReady
func goGTKAsyncReady(source *C.GObject, result *C.GAsyncResult, data unsafe.Pointer) {
	invokeNativeAsync(source, result, data)
}

//export goGTKBusAppeared
func goGTKBusAppeared(connection *C.GDBusConnection, name, owner *C.char, data unsafe.Pointer) {
	nativeCallback(data).(*nativeBusWatch).appeared()
}

//export goGTKBusVanished
func goGTKBusVanished(connection *C.GDBusConnection, name *C.char, data unsafe.Pointer) {
	nativeCallback(data).(*nativeBusWatch).vanished()
}

//export goGTKCallbackFree
func goGTKCallbackFree(data unsafe.Pointer, closure *C.GClosure) { freeNativeCallback(data) }

//export goGTKCloseRequest
func goGTKCloseRequest(instance, data unsafe.Pointer) C.gboolean {
	return invokeNativeCloseRequest(data)
}

//export goGTKDBusMethod
func goGTKDBusMethod(connection *C.GDBusConnection, sender, path, iface, method *C.char, parameters *C.GVariant, invocation *C.GDBusMethodInvocation, data unsafe.Pointer) {
	invokeNativeBusMethod(goString(method), parameters, invocation, data)
}

//export goGTKDBusProperty
func goGTKDBusProperty(connection *C.GDBusConnection, sender, path, iface, name *C.char, failure **C.GError, data unsafe.Pointer) *C.GVariant {
	return invokeNativeBusProperty(goString(name), data)
}

//export goGTKDBusSignal
func goGTKDBusSignal(connection *C.GDBusConnection, sender, path, iface, signal *C.char, parameters *C.GVariant, data unsafe.Pointer) {
	invokeNativeDBusSignal(signal, parameters, data)
}

//export goGTKDestroy
func goGTKDestroy(data unsafe.Pointer) { freeNativeCallback(data) }

//export goGTKDragPrepare
func goGTKDragPrepare(instance unsafe.Pointer, x, y C.double, data unsafe.Pointer) *C.GdkContentProvider {
	return nativeCallback(data).(func(float64, float64) *gdkContentProvider)(float64(x), float64(y))
}

//export goGTKDraw
func goGTKDraw(area *C.GtkDrawingArea, context *C.cairo_t, width, height C.int, data unsafe.Pointer) {
	nativeCallback(data).(func(*cairoContext, int, int))(context, int(width), int(height))
}

//export goGTKDropAccept
func goGTKDropAccept(instance unsafe.Pointer, drop *C.GdkDrop, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(*gdkDrop) bool)(drop))
}

//export goGTKDropEnter
func goGTKDropEnter(instance unsafe.Pointer, x, y C.double, data unsafe.Pointer) C.GdkDragAction {
	return nativeCallback(data).(func(float64, float64) gdkDragAction)(float64(x), float64(y))
}

//export goGTKDropMotion
func goGTKDropMotion(instance unsafe.Pointer, drop *C.GdkDrop, x, y C.double, data unsafe.Pointer) C.GdkDragAction {
	return nativeCallback(data).(func(*gdkDrop, float64, float64) gdkDragAction)(drop, float64(x), float64(y))
}

//export goGTKDropPerform
func goGTKDropPerform(instance unsafe.Pointer, drop *C.GdkDrop, x, y C.double, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(*gdkDrop, float64, float64) bool)(drop, float64(x), float64(y)))
}

//export goGTKEqualSort
func goGTKEqualSort(a, b, data unsafe.Pointer) C.int { return nativeEqualSort() }

//export goGTKImageClass
func goGTKImageClass(class, data unsafe.Pointer) {
	initializeNativeImageClass((*gtkWidgetClass)(class))
}

//export goGTKImageMeasure
func goGTKImageMeasure(widget *C.GtkWidget, orientation C.GtkOrientation, forSize C.int, minimum, natural, minimumBaseline, naturalBaseline *C.int) {
	low, high := measureNativeImage(widget, orientation)
	*minimum, *natural, *minimumBaseline, *naturalBaseline = C.int(low), C.int(high), -1, -1
}

//export goGTKImageSnapshot
func goGTKImageSnapshot(widget *C.GtkWidget, snapshot *C.GtkSnapshot) {
	snapshotNativeImage(widget, snapshot)
}

//export goGTKIntSignal
func goGTKIntSignal(instance unsafe.Pointer, value C.int, data unsafe.Pointer) {
	invokeNativeSignal(data)
}

//export goGTKKeyReleased
func goGTKKeyReleased(instance unsafe.Pointer, keyval, keycode C.uint, state C.GdkModifierType, data unsafe.Pointer) {
	nativeCallback(data).(func(uint32, uint32, gdkModifierType))(uint32(keyval), uint32(keycode), state)
}

//export goGTKKeySignal
func goGTKKeySignal(instance unsafe.Pointer, keyval, keycode C.uint, state C.GdkModifierType, data unsafe.Pointer) C.gboolean {
	return invokeNativeKey(uint32(keyval), uint32(keycode), state, data)
}

//export goGTKLayoutAllocate
func goGTKLayoutAllocate(widget *C.GtkWidget, width, height, baseline C.int) {
	allocateNativeLayout(widget, int(width), int(height))
}

//export goGTKLayoutClass
func goGTKLayoutClass(class, data unsafe.Pointer) {
	initializeNativeLayoutClass((*gtkWidgetClass)(class))
}

//export goGTKLayoutMeasure
func goGTKLayoutMeasure(widget *C.GtkWidget, orientation C.GtkOrientation, forSize C.int, minimum, natural, minimumBaseline, naturalBaseline *C.int) {
	low, high := measureNativeLayout(widget, orientation)
	*minimum, *natural, *minimumBaseline, *naturalBaseline = C.int(low), C.int(high), -1, -1
}

//export goGTKLayoutSnapshot
func goGTKLayoutSnapshot(widget *C.GtkWidget, snapshot *C.GtkSnapshot) {
	snapshotNativeLayout(widget, snapshot)
}

//export goGTKLegacyEvent
func goGTKLegacyEvent(instance unsafe.Pointer, event *C.GdkEvent, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(*gdkEvent) bool)(event))
}

//export goGTKListItem
func goGTKListItem(instance unsafe.Pointer, item *C.GtkListItem, data unsafe.Pointer) {
	nativeCallback(data).(func(*gtkListItem))(item)
}

//export goGTKNotify
func goGTKNotify(instance unsafe.Pointer, property *C.GParamSpec, data unsafe.Pointer) {
	invokeNativeSignal(data)
}

//export goGTKObjectSignal
func goGTKObjectSignal(instance, object, data unsafe.Pointer) {
	nativeCallback(data).(func(unsafe.Pointer))(object)
}

//export goGTKOverlayPosition
func goGTKOverlayPosition(instance unsafe.Pointer, child *C.GtkWidget, allocation *C.GdkRectangle, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(*gtkWidget, *gdkRectangle) bool)(child, allocation))
}

//export goGTKPressed
func goGTKPressed(instance unsafe.Pointer, presses C.int, x, y C.double, data unsafe.Pointer) {
	nativeCallback(data).(func(float64, float64))(float64(x), float64(y))
}

//export goGTKPrintBegin
func goGTKPrintBegin(operation *C.GtkPrintOperation, context *C.GtkPrintContext, data unsafe.Pointer) {
	beginNativePrint(operation, context, data)
}

//export goGTKPrintDraw
func goGTKPrintDraw(operation *C.GtkPrintOperation, context *C.GtkPrintContext, page C.int, data unsafe.Pointer) {
	drawNativePrint(operation, context, int(page), data)
}

//export goGTKRangeValue
func goGTKRangeValue(instance unsafe.Pointer, scroll C.GtkScrollType, value C.double, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(float64) bool)(float64(value)))
}

//export goGTKScrollSignal
func goGTKScrollSignal(instance unsafe.Pointer, x, y C.double, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(float64, float64) bool)(float64(x), float64(y)))
}

//export goGTKSelectionSignal
func goGTKSelectionSignal(instance unsafe.Pointer, position, count C.uint, data unsafe.Pointer) {
	invokeNativeSignal(data)
}

//export goGTKSignal
func goGTKSignal(instance, data unsafe.Pointer) { invokeNativeSignal(data) }

//export goGTKSource
func goGTKSource(data unsafe.Pointer) C.gboolean { return invokeNativeSource(data) }

//export goGTKStringSignal
func goGTKStringSignal(instance unsafe.Pointer, text *C.char, data unsafe.Pointer) {
	nativeCallback(data).(func(string))(goString(text))
}

//export goGTKTabSwitch
func goGTKTabSwitch(instance, page unsafe.Pointer, position C.uint, data unsafe.Pointer) {
	nativeCallback(data).(func())()
}

//export goGTKTextInsert
func goGTKTextInsert(instance unsafe.Pointer, location *C.GtkTextIter, text *C.char, length C.int, data unsafe.Pointer) {
	nativeCallback(data).(func(string))(string(unsafe.Slice((*byte)(unsafe.Pointer(text)), int(length))))
}

//export goGTKTick
func goGTKTick(widget *C.GtkWidget, clock *C.GdkFrameClock, data unsafe.Pointer) C.gboolean {
	return invokeNativeTick(clock, data)
}

//export goGTKTreeChildren
func goGTKTreeChildren(item, data unsafe.Pointer) *C.GListModel {
	return nativeCallback(data).(func(unsafe.Pointer) *gListModel)(item)
}

//export goGTKUIntSignal
func goGTKUIntSignal(instance unsafe.Pointer, position C.uint, data unsafe.Pointer) {
	nativeCallback(data).(func(uint32))(uint32(position))
}

//export goGTKValueDrop
func goGTKValueDrop(instance unsafe.Pointer, value *C.GValue, x, y C.double, data unsafe.Pointer) C.gboolean {
	return nativeBool(nativeCallback(data).(func(*gValue, float64, float64) bool)(value, float64(x), float64(y)))
}

//export goGTKXYSignal
func goGTKXYSignal(instance unsafe.Pointer, x, y C.double, data unsafe.Pointer) {
	nativeCallback(data).(func(float64, float64))(float64(x), float64(y))
}
