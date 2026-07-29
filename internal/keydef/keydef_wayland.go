//go:build linux

package keydef

import (
	"fmt"
	"strings"

	"github.com/happyboard/happyboard/internal/hook"
)

const (
	evdevKeyLeftCtrl  uint16 = 29
	evdevKeyLeftShift uint16 = 42
	evdevKeyLeftAlt   uint16 = 56
	evdevKeyLeftMeta  uint16 = 125
)

func EvdevCode(name string) (uint16, error) {
	m, ok := keyMap[strings.ToLower(name)]
	if !ok {
		return 0, fmt.Errorf("unknown key name: %q", name)
	}
	return m.EvdevCode, nil
}

func EvdevModifierCode(mod hook.Modifier) (uint16, bool) {
	switch mod {
	case hook.ModCtrl:
		return evdevKeyLeftCtrl, true
	case hook.ModShift:
		return evdevKeyLeftShift, true
	case hook.ModAlt:
		return evdevKeyLeftAlt, true
	case hook.ModSuper:
		return evdevKeyLeftMeta, true
	default:
		return 0, false
	}
}

func EvdevCodeToName() map[uint16]string {
	m := make(map[uint16]string, len(keyMap))
	for name, km := range keyMap {
		if _, exists := m[km.EvdevCode]; !exists {
			m[km.EvdevCode] = name
		}
	}
	return m
}
