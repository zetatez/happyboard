//go:build darwin && cgo

package keydef

import (
	"fmt"
	"strings"
)

func MacKeyCode(name string) (int, error) {
	m, ok := keyMap[strings.ToLower(name)]
	if !ok {
		return 0, fmt.Errorf("unknown key name: %q", name)
	}
	return m.MacCode, nil
}
