//go:build windows

package main

import (
	"github.com/happyboard/happyboard/internal/hook"
	winhook "github.com/happyboard/happyboard/internal/hook/windows"
	"github.com/happyboard/happyboard/internal/inject"
	wininj "github.com/happyboard/happyboard/internal/inject/windows"
	"github.com/happyboard/happyboard/internal/window"
	winwin "github.com/happyboard/happyboard/internal/window/windows"
	log "github.com/sirupsen/logrus"
)

func createWindowsComponents() (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	hk := winhook.New()
	inj := wininj.New()
	mon := winwin.New()
	log.Info("using Windows backend (Win32)")
	return hk, inj, mon, nil
}
