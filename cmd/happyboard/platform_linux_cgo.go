//go:build linux && cgo

package main

import (
	"fmt"
	"os"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/hook"
	x11hook "github.com/happyboard/happyboard/internal/hook/x11"
	"github.com/happyboard/happyboard/internal/inject"
	x11inj "github.com/happyboard/happyboard/internal/inject/x11"
	"github.com/happyboard/happyboard/internal/window"
	x11win "github.com/happyboard/happyboard/internal/window/x11"
	log "github.com/sirupsen/logrus"
)

func createLinuxComponents(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	backend := cfg.Platform.Linux.Backend
	if backend == "auto" {
		backend = detectLinuxBackend()
	}

	if backend == "x11" {
		return newX11Components(cfg)
	}

	log.Info("falling back to wayland backend")
	return createWaylandComponents(cfg)
}

func newX11Components(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	hk, err := x11hook.NewHook()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("x11 hook: %w", err)
	}

	inj, err := x11inj.NewInjector()
	if err != nil {
		hk.Stop()
		return nil, nil, nil, fmt.Errorf("x11 injector: %w", err)
	}

	mon, err := x11win.NewMonitor()
	if err != nil {
		hk.Stop()
		inj.Close()
		return nil, nil, nil, fmt.Errorf("x11 monitor: %w", err)
	}

	log.Info("using X11 backend")
	return hk, inj, mon, nil
}

func detectLinuxBackend() string {
	session := os.Getenv("XDG_SESSION_TYPE")
	if session == "wayland" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "wayland"
	}
	return "x11"
}
