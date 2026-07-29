//go:build windows

package keydef

import (
	"fmt"
	"strings"
)

func WinVKCode(name string) (uint16, error) {
	m, ok := keyMap[strings.ToLower(name)]
	if !ok {
		return 0, fmt.Errorf("unknown key name: %q", name)
	}
	return m.WinVK, nil
}
