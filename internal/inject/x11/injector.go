//go:build linux && cgo

package x11

//#cgo LDFLAGS: -lX11 -lXtst -lXext
//#include <X11/Xlib.h>
//#include <X11/Xutil.h>
//#include <X11/keysym.h>
//#include <X11/extensions/XTest.h>
import "C"

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/happyboard/happyboard/internal/keydef"
)

type Injector struct {
	display *C.Display
}

func NewInjector() (*Injector, error) {
	d := C.XOpenDisplay(nil)
	if d == nil {
		return nil, fmt.Errorf("cannot open X display")
	}
	return &Injector{display: d}, nil
}

func (i *Injector) Close() error {
	if i.display != nil {
		C.XCloseDisplay(i.display)
		i.display = nil
	}
	return nil
}

func (i *Injector) keyNameToCode(name string) (C.KeyCode, error) {
	sym, err := keydef.X11KeySym(name)
	if err != nil {
		return 0, err
	}
	code := C.XKeysymToKeycode(i.display, C.KeySym(sym))
	if code == 0 {
		return 0, fmt.Errorf("no keycode for key %q", name)
	}
	return code, nil
}

func (i *Injector) PressKey(keyName string) error {
	code, err := i.keyNameToCode(keyName)
	if err != nil {
		return err
	}
	C.XTestFakeKeyEvent(i.display, C.uint(code), 1, 0)
	C.XFlush(i.display)
	return nil
}

func (i *Injector) ReleaseKey(keyName string) error {
	code, err := i.keyNameToCode(keyName)
	if err != nil {
		return err
	}
	C.XTestFakeKeyEvent(i.display, C.uint(code), 0, 0)
	C.XFlush(i.display)
	return nil
}

func (i *Injector) TapKey(keyName string) error {
	if err := i.PressKey(keyName); err != nil {
		return err
	}
	return i.ReleaseKey(keyName)
}

func (i *Injector) TypeText(text string, delayMs int) error {
	prevPrimary, _ := exec.Command("xclip", "-o", "-selection", "primary").Output()
	prevClipboard, _ := exec.Command("xclip", "-o", "-selection", "clipboard").Output()

	setCmd := exec.Command("xclip", "-selection", "primary")
	setCmd.Stdin = strings.NewReader(text)
	if err := setCmd.Run(); err != nil {
		return exec.Command("xdotool", "type", "--", text).Run()
	}

	clipCmd := exec.Command("xclip", "-selection", "clipboard")
	clipCmd.Stdin = strings.NewReader(text)
	clipCmd.Run()

	time.Sleep(30 * time.Millisecond)
	exec.Command("xdotool", "key", "Shift+Insert").Run()
	time.Sleep(50 * time.Millisecond)

	if len(prevPrimary) > 0 {
		restoreCmd := exec.Command("xclip", "-selection", "primary")
		restoreCmd.Stdin = bytes.NewReader(prevPrimary)
		restoreCmd.Run()
	}
	if len(prevClipboard) > 0 {
		restoreCmd := exec.Command("xclip", "-selection", "clipboard")
		restoreCmd.Stdin = bytes.NewReader(prevClipboard)
		restoreCmd.Run()
	}
	return nil
}
