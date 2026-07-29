//go:build darwin && cgo

package darwin

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>

extern CGEventRef goEventCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *userInfo);

static CGEventTapRef hbCreateEventTap(void) {
	return CGEventTapCreate(
		kCGSessionEventTap,
		kCGHeadInsertEventTap,
		kCGEventTapOptionDefault,
		CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventKeyUp),
		(CGEventTapCallBack)goEventCallback,
		NULL
	);
}

static CFRunLoopSourceRef hbCreateTapSource(CGEventTapRef tap) {
	return CGEventTapCreateRunLoopSource(kCFAllocatorDefault, tap, false);
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	log "github.com/sirupsen/logrus"
)

var activeHook atomic.Pointer[Hook]

type Hook struct {
	mu          sync.RWMutex
	handler     func(hook.KeyEvent) bool
	paused      bool
	toggleCombo hook.KeyCombo
	combos      []hook.KeyCombo

	codeToName map[int]string

	loop   C.CFRunLoopRef
	tap    C.CGEventTapRef
	source C.CFRunLoopSourceRef
	wg     sync.WaitGroup
}

func New() *Hook {
	return &Hook{
		codeToName: buildCodeToName(),
	}
}

func (h *Hook) SetHandler(handler func(hook.KeyEvent) bool) {
	h.mu.Lock()
	h.handler = handler
	h.mu.Unlock()
}

func buildCodeToName() map[int]string {
	m := make(map[int]string, 128)
	for name, km := range keydef.GetKeyMap() {
		if km.MacCode >= 0 {
			if _, exists := m[km.MacCode]; !exists {
				m[km.MacCode] = name
			}
		}
	}
	return m
}

func (h *Hook) Start() error {
	h.mu.Lock()
	h.mu.Unlock()

	tap := C.hbCreateEventTap()
	if tap == nil {
		return fmt.Errorf("failed to create CGEventTap (grant Accessibility permission to this process)")
	}

	resultCh := make(chan error, 1)
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		loop := C.CFRunLoopGetCurrent()
		source := C.hbCreateTapSource(tap)
		if source == nil {
			C.CFRelease(tap)
			resultCh <- fmt.Errorf("failed to create CFRunLoopSource for event tap")
			return
		}

		C.CFRunLoopAddSource(loop, source, C.kCFRunLoopCommonModes)

		h.mu.Lock()
		h.loop = loop
		h.tap = tap
		h.source = source
		h.mu.Unlock()

		activeHook.Store(h)
		resultCh <- nil

		log.Info("darwin hook: CGEventTap running")
		C.CFRunLoopRun()
		log.Info("darwin hook: CGEventTap stopped")

		h.mu.Lock()
		C.CFRunLoopRemoveSource(loop, source, C.kCFRunLoopCommonModes)
		C.CFRelease(source)
		C.CGEventTapEnable(tap, 0)
		C.CFRelease(tap)
		h.loop = nil
		h.tap = nil
		h.source = nil
		h.mu.Unlock()
	}()

	return <-resultCh
}

func (h *Hook) Stop() error {
	activeHook.Store(nil)
	h.mu.Lock()
	loop := h.loop
	h.mu.Unlock()
	if loop != nil {
		C.CFRunLoopStop(loop)
	}
	h.wg.Wait()
	return nil
}

func (h *Hook) SetToggleGrab(combo hook.KeyCombo) error {
	h.mu.Lock()
	h.toggleCombo = combo
	h.mu.Unlock()
	return nil
}

func (h *Hook) UpdateGrabs(combos []hook.KeyCombo) error {
	h.mu.Lock()
	h.combos = combos
	h.mu.Unlock()
	return nil
}

func (h *Hook) Pause() error {
	h.mu.Lock()
	h.paused = true
	h.mu.Unlock()
	log.Info("darwin hook: paused")
	return nil
}

func (h *Hook) Resume() error {
	h.mu.Lock()
	h.paused = false
	h.mu.Unlock()
	log.Info("darwin hook: resumed")
	return nil
}

func (h *Hook) handleEvent(ev hook.KeyEvent) bool {
	h.mu.RLock()
	handler := h.handler
	paused := h.paused
	toggleCombo := h.toggleCombo
	h.mu.RUnlock()

	if paused {
		if !isToggleMatch(ev, toggleCombo) {
			return false
		}
		if handler != nil {
			return handler(ev)
		}
		return false
	}

	if handler != nil {
		return handler(ev)
	}
	}
	return false
}

func isToggleMatch(ev hook.KeyEvent, toggle hook.KeyCombo) bool {
	if toggle.Key == "" {
		return false
	}
	combo := hook.KeyCombo{Modifiers: ev.Modifiers, Key: ev.KeyName}
	n1, err := keydef.NormalizeCombo(combo)
	if err != nil {
		return false
	}
	n2, err := keydef.NormalizeCombo(toggle)
	if err != nil {
		return false
	}
	return keydef.ComboKey(n1) == keydef.ComboKey(n2)
}
