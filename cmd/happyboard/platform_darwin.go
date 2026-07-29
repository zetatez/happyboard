//go:build darwin && cgo

package main

import (
	"github.com/happyboard/happyboard/internal/hook"
	darwinhook "github.com/happyboard/happyboard/internal/hook/darwin"
	"github.com/happyboard/happyboard/internal/inject"
	darwininj "github.com/happyboard/happyboard/internal/inject/darwin"
	"github.com/happyboard/happyboard/internal/window"
	darwinwin "github.com/happyboard/happyboard/internal/window/darwin"
	log "github.com/sirupsen/logrus"
)

func createDarwinComponents() (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	hk := darwinhook.New()
	inj := darwininj.New()
	mon := darwinwin.New()
	log.Info("using macOS backend (CGEventTap + CGEventPost + NSWorkspace)")
	return hk, inj, mon, nil
}
