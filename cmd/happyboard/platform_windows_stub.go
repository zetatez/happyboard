//go:build !windows

package main

import (
	"fmt"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/window"
)

func createWindowsComponents() (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	return nil, nil, nil, fmt.Errorf("windows backend not supported on this platform")
}
