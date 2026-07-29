//go:build linux

package main

import (
	"fmt"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/hook"
	waylandhook "github.com/happyboard/happyboard/internal/hook/wayland"
	"github.com/happyboard/happyboard/internal/inject"
	waylandinj "github.com/happyboard/happyboard/internal/inject/wayland"
	"github.com/happyboard/happyboard/internal/window"
	waylandwin "github.com/happyboard/happyboard/internal/window/wayland"
	log "github.com/sirupsen/logrus"
)

func createWaylandComponents(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	inj, err := waylandinj.New()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("wayland injector: %w", err)
	}

	var handler func(hook.KeyEvent) bool
	hk := waylandhook.New(handler, inj)

	mon := waylandwin.New()

	log.Info("using Wayland backend (evdev + uinput)")
	return hk, inj, mon, nil
}
