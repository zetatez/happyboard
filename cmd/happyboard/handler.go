//go:build linux || darwin || windows

package main

import (
	"github.com/happyboard/happyboard/internal/dispatch"
	"github.com/happyboard/happyboard/internal/hook"
)

func setHookHandler(hk hook.KeyboardHook, d *dispatch.Dispatcher) {
	if wh, ok := hk.(interface {
		SetHandler(func(hook.KeyEvent) bool)
	}); ok {
		wh.SetHandler(func(ev hook.KeyEvent) bool {
			return d.HandleEvent(ev)
		})
	}
}
