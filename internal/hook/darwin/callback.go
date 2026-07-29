//go:build darwin && cgo

package darwin

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
*/
import "C"

import (
	"unsafe"

	"github.com/happyboard/happyboard/internal/hook"
)

//export goEventCallback
func goEventCallback(_ C.CGEventTapProxy, eventType C.CGEventType, event C.CGEventRef, _ unsafe.Pointer) C.CGEventRef {
	h := activeHook.Load()
	if h == nil {
		return event
	}

	if eventType == C.CGEventType(C.kCGEventTapDisabledByTimeout) ||
		eventType == C.CGEventType(C.kCGEventTapDisabledByUserInput) {
		h.mu.RLock()
		tap := h.tap
		h.mu.RUnlock()
		if tap != nil {
			C.CGEventTapEnable(tap, 1)
		}
		return event
	}

	if eventType != C.CGEventType(C.kCGEventKeyDown) && eventType != C.CGEventType(C.kCGEventKeyUp) {
		return event
	}

	keyCode := int(C.CGEventGetIntegerValueField(event, C.CGEventField(C.kCGKeyboardEventKeycode)))
	flags := C.CGEventGetFlags(event)

	keyName, ok := h.codeToName[keyCode]
	if !ok {
		return event
	}

	evType := hook.KeyPress
	if eventType == C.CGEventType(C.kCGEventKeyUp) {
		evType = hook.KeyRelease
	}

	ev := hook.KeyEvent{
		Type:      evType,
		KeyName:   keyName,
		Modifiers: flagsToModifiers(flags),
	}

	if h.handleEvent(ev) {
		return 0
	}
	return event
}

func flagsToModifiers(flags C.CGEventFlags) []hook.Modifier {
	var mods []hook.Modifier
	f := uint64(flags)
	if f&uint64(C.kCGEventFlagMaskCommand) != 0 {
		mods = append(mods, hook.ModSuper)
	}
	if f&uint64(C.kCGEventFlagMaskShift) != 0 {
		mods = append(mods, hook.ModShift)
	}
	if f&uint64(C.kCGEventFlagMaskControl) != 0 {
		mods = append(mods, hook.ModCtrl)
	}
	if f&uint64(C.kCGEventFlagMaskAlternate) != 0 {
		mods = append(mods, hook.ModAlt)
	}
	return mods
}
