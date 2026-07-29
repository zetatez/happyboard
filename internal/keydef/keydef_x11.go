//go:build linux && cgo

package keydef

import (
	"fmt"
	"strings"

	"github.com/happyboard/happyboard/internal/hook"
)

const (
	x11ShiftMask   uint = 1 << 0
	x11ControlMask uint = 1 << 2
	x11Mod1Mask    uint = 1 << 3
	x11Mod3Mask    uint = 1 << 5
	x11Mod4Mask    uint = 1 << 6
)

func X11KeySym(name string) (int, error) {
	m, ok := keyMap[strings.ToLower(name)]
	if !ok {
		return 0, fmt.Errorf("unknown key name: %q", name)
	}
	return m.X11Sym, nil
}

func X11ModifierMask(mods []hook.Modifier) uint {
	var mask uint
	for _, mod := range mods {
		switch mod {
		case hook.ModCtrl:
			mask |= x11ControlMask
		case hook.ModShift:
			mask |= x11ShiftMask
		case hook.ModAlt:
			mask |= x11Mod1Mask
		case hook.ModSuper:
			mask |= x11Mod4Mask
		case hook.ModHyper:
			mask |= x11Mod3Mask
		}
	}
	return mask
}
