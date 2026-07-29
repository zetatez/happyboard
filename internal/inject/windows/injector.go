//go:build windows

package windows

import (
	"fmt"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/happyboard/happyboard/internal/keydef"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	INPUT_KEYBOARD    = 1
	KEYEVENTF_KEYUP   = 0x0002
	KEYEVENTF_UNICODE = 0x0004

	vkReturn = 0x0D
)

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// The trailing pad makes INPUT the same size as the Win32 INPUT union, whose
// size is dictated by MOUSEINPUT. This is required: SendInput rejects a cbSize
// that does not equal sizeof(INPUT).
type INPUT struct {
	Type uint32
	Ki   KEYBDINPUT
	_    [8]byte
}

var pSendInput = windows.NewLazySystemDLL("user32.dll").NewProc("SendInput")

type Injector struct{}

func New() *Injector {
	return &Injector{}
}

func sendInput(inputs []INPUT) error {
	if len(inputs) == 0 {
		return nil
	}
	n, _, err := pSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(INPUT{}),
	)
	if n != uintptr(len(inputs)) {
		return fmt.Errorf("SendInput failed: %w", err)
	}
	return nil
}

func (inj *Injector) TapKey(keyName string) error {
	vk, err := keydef.WinVKCode(keyName)
	if err != nil {
		return err
	}
	return sendInput([]INPUT{
		{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: vk}},
		{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: vk, DwFlags: KEYEVENTF_KEYUP}},
	})
}

func (inj *Injector) TypeText(text string, delayMs int) error {
	delay := time.Duration(delayMs) * time.Millisecond
	if delay == 0 {
		delay = 10 * time.Millisecond
	}
	for _, r := range text {
		if err := inj.typeChar(r); err != nil {
			log.Warnf("windows injector: skipping character %q: %v", r, err)
			continue
		}
		time.Sleep(delay)
	}
	return nil
}

func (inj *Injector) typeChar(r rune) error {
	if r == '\n' {
		return sendInput([]INPUT{
			{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: vkReturn}},
			{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: vkReturn, DwFlags: KEYEVENTF_KEYUP}},
		})
	}

	codes := utf16.Encode([]rune{r})
	inputs := make([]INPUT, 0, len(codes)*2)
	for _, c := range codes {
		inputs = append(inputs, INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WScan: c, DwFlags: KEYEVENTF_UNICODE}})
	}
	for i := len(codes) - 1; i >= 0; i-- {
		inputs = append(inputs, INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WScan: codes[i], DwFlags: KEYEVENTF_UNICODE | KEYEVENTF_KEYUP}})
	}
	return sendInput(inputs)
}

func (inj *Injector) Close() error {
	return nil
}
