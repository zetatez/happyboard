//go:build !linux

package main

import (
	"fmt"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/window"
)

func createLinuxComponents(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	return nil, nil, nil, fmt.Errorf("linux backend not supported on this platform")
}
