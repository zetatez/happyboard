//go:build darwin && !cgo

package main

import (
	"fmt"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/window"
)

func createDarwinComponents() (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	return nil, nil, nil, fmt.Errorf("macOS backend requires CGO; rebuild with CGO_ENABLED=1")
}
