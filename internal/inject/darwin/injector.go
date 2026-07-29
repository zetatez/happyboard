//go:build darwin && cgo

package darwin

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static void hbPostKey(CGKeyCode keycode, int down) {
	CGEventRef e = CGEventCreateKeyboardEvent(NULL, keycode, down != 0);
	if (e) {
		CGEventPost(kCGHIDEventTap, e);
		CFRelease(e);
	}
}

static void hbPostUnicode(UniChar ch) {
	CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0, true);
	if (down) {
		CGEventKeyboardSetUnicodeString(down, 1, &ch);
		CGEventPost(kCGHIDEventTap, down);
		CFRelease(down);
	}
	CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0, false);
	if (up) {
		CGEventKeyboardSetUnicodeString(up, 1, &ch);
		CGEventPost(kCGHIDEventTap, up);
		CFRelease(up);
	}
}
*/
import "C"

import (
	"time"

	"github.com/happyboard/happyboard/internal/keydef"
)

type Injector struct{}

func New() *Injector {
	return &Injector{}
}

func (inj *Injector) TapKey(keyName string) error {
	code, err := keydef.MacKeyCode(keyName)
	if err != nil {
		return err
	}
	C.hbPostKey(C.CGKeyCode(code), 1)
	time.Sleep(12 * time.Millisecond)
	C.hbPostKey(C.CGKeyCode(code), 0)
	return nil
}

func (inj *Injector) TypeText(text string, delayMs int) error {
	delay := time.Duration(delayMs) * time.Millisecond
	if delay == 0 {
		delay = 10 * time.Millisecond
	}
	for _, r := range text {
		if r == '\n' {
			if code, err := keydef.MacKeyCode("enter"); err == nil {
				C.hbPostKey(C.CGKeyCode(code), 1)
				time.Sleep(12 * time.Millisecond)
				C.hbPostKey(C.CGKeyCode(code), 0)
			}
		} else {
			C.hbPostUnicode(C.UniChar(r))
		}
		time.Sleep(delay)
	}
	return nil
}

func (inj *Injector) Close() error {
	return nil
}
