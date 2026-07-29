//go:build linux && !cgo

package main

import (
	"os"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/window"
)

func createLinuxComponents(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	return createWaylandComponents(cfg)
}

func detectLinuxBackend() string {
	session := os.Getenv("XDG_SESSION_TYPE")
	if session == "wayland" {
		return "wayland"
	}
	return "wayland"
}
