//go:build !linux && !darwin && !windows

package main

import (
	"github.com/happyboard/happyboard/internal/dispatch"
	"github.com/happyboard/happyboard/internal/hook"
)

func setHookHandler(_ hook.KeyboardHook, _ *dispatch.Dispatcher) {}
