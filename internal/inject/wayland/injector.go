//go:build linux

package wayland

import (
	"fmt"
	"time"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	evdev "github.com/holoplot/go-evdev"
	"github.com/sirupsen/logrus"
)

type Injector struct {
	dev *evdev.InputDevice
}

func New() (*Injector, error) {
	codes := make([]evdev.EvCode, 256)
	for i := 0; i < 256; i++ {
		codes[i] = evdev.EvCode(i)
	}
	caps := map[evdev.EvType][]evdev.EvCode{
		evdev.EV_KEY: codes,
		evdev.EV_SYN: {evdev.SYN_REPORT},
	}
	dev, err := evdev.CreateDevice("happyboard-virtual-keyboard", evdev.InputID{
		BusType: evdev.BUS_VIRTUAL,
		Vendor:  0x1234,
		Product: 0x5678,
		Version: 1,
	}, caps)
	if err != nil {
		return nil, fmt.Errorf("failed to create uinput device: %w", err)
	}
	logrus.Info("wayland injector: virtual keyboard created")
	return &Injector{dev: dev}, nil
}

func (inj *Injector) writeEvent(typ evdev.EvType, code uint16, value int32) error {
	event := evdev.InputEvent{
		Type:  typ,
		Code:  evdev.EvCode(code),
		Value: value,
	}
	if err := inj.dev.WriteOne(&event); err != nil {
		return err
	}
	syn := evdev.InputEvent{
		Type:  evdev.EV_SYN,
		Code:  evdev.SYN_REPORT,
		Value: 0,
	}
	return inj.dev.WriteOne(&syn)
}

func (inj *Injector) InjectEvent(code uint16, value int32) error {
	return inj.writeEvent(evdev.EV_KEY, code, value)
}

func (inj *Injector) TapKey(keyName string) error {
	code, err := keydef.EvdevCode(keyName)
	if err != nil {
		return err
	}
	if err := inj.writeEvent(evdev.EV_KEY, code, 1); err != nil {
		return err
	}
	time.Sleep(12 * time.Millisecond)
	return inj.writeEvent(evdev.EV_KEY, code, 0)
}

func (inj *Injector) TypeText(text string, delayMs int) error {
	delay := time.Duration(delayMs) * time.Millisecond
	if delay == 0 {
		delay = 10 * time.Millisecond
	}
	for _, r := range text {
		if err := inj.typeChar(r); err != nil {
			logrus.Warnf("wayland injector: skipping character %q: %v", r, err)
			continue
		}
		time.Sleep(delay)
	}
	return nil
}

func (inj *Injector) typeChar(r rune) error {
	if r > 127 {
		return fmt.Errorf("non-ASCII character not supported via uinput: %q (U+%04X)", r, r)
	}

	needShift := false
	keyName := ""

	switch {
	case r >= 'a' && r <= 'z':
		keyName = string(r)
	case r >= 'A' && r <= 'Z':
		keyName = string(r + 32)
		needShift = true
	case r >= '0' && r <= '9':
		keyName = string(r)
	case r == ' ':
		keyName = "space"
	case r == '\n':
		keyName = "enter"
	case r == '\t':
		keyName = "tab"
	case r == '\b':
		keyName = "backspace"
	default:
		keyName, needShift = shiftedChar(r)
		if keyName == "" {
			return fmt.Errorf("unsupported character: %q", r)
		}
	}

	code, err := keydef.EvdevCode(keyName)
	if err != nil {
		return err
	}

	if needShift {
		shiftCode, _ := keydef.EvdevModifierCode(hook.ModShift)
		if err := inj.writeEvent(evdev.EV_KEY, shiftCode, 1); err != nil {
			return err
		}
	}
	if err := inj.writeEvent(evdev.EV_KEY, code, 1); err != nil {
		return err
	}
	if err := inj.writeEvent(evdev.EV_KEY, code, 0); err != nil {
		return err
	}
	if needShift {
		shiftCode, _ := keydef.EvdevModifierCode(hook.ModShift)
		if err := inj.writeEvent(evdev.EV_KEY, shiftCode, 0); err != nil {
			return err
		}
	}
	return nil
}

func shiftedChar(r rune) (keyName string, needShift bool) {
	switch r {
	case '-':
		return "minus", false
	case '=':
		return "equal", false
	case '[':
		return "left_bracket", false
	case ']':
		return "right_bracket", false
	case '\\':
		return "backslash", false
	case ';':
		return "semicolon", false
	case '\'':
		return "quote", false
	case '`':
		return "backtick", false
	case ',':
		return "comma", false
	case '.':
		return "period", false
	case '/':
		return "slash", false
	case '!':
		return "1", true
	case '@':
		return "2", true
	case '#':
		return "3", true
	case '$':
		return "4", true
	case '%':
		return "5", true
	case '^':
		return "6", true
	case '&':
		return "7", true
	case '*':
		return "8", true
	case '(':
		return "9", true
	case ')':
		return "0", true
	case '_':
		return "minus", true
	case '+':
		return "equal", true
	case '{':
		return "left_bracket", true
	case '}':
		return "right_bracket", true
	case '|':
		return "backslash", true
	case ':':
		return "semicolon", true
	case '"':
		return "quote", true
	case '~':
		return "backtick", true
	case '<':
		return "comma", true
	case '>':
		return "period", true
	case '?':
		return "slash", true
	default:
		return "", false
	}
}

func (inj *Injector) Close() error {
	if inj.dev == nil {
		return nil
	}
	if err := evdev.DestroyDevice(inj.dev); err != nil {
		logrus.Warnf("wayland injector: destroy device failed: %v", err)
	}
	return inj.dev.Close()
}
