//go:build linux

package wayland

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	evdev "github.com/holoplot/go-evdev"
	"github.com/sirupsen/logrus"
)

type EventInjector interface {
	InjectEvent(code uint16, value int32) error
}

type Hook struct {
	handler  func(hook.KeyEvent) bool
	injector EventInjector

	devices []*evdev.InputDevice
	wg      sync.WaitGroup
	stopped atomic.Bool

	combos      []hook.KeyCombo
	toggleCombo hook.KeyCombo

	stateMu     sync.Mutex
	modState    map[uint16]bool
	suppressed  map[uint16]bool
	modMap      map[uint16]hook.Modifier
	evdevToName map[uint16]string
}

func New(handler func(hook.KeyEvent) bool, injector EventInjector) *Hook {
	return &Hook{
		handler:     handler,
		injector:    injector,
		modState:    make(map[uint16]bool),
		suppressed:  make(map[uint16]bool),
		modMap:      buildModMap(),
		evdevToName: keydef.EvdevCodeToName(),
	}
}

func (h *Hook) SetHandler(handler func(hook.KeyEvent) bool) {
	h.handler = handler
}

func buildModMap() map[uint16]hook.Modifier {
	m := map[uint16]hook.Modifier{
		29:  hook.ModCtrl,
		97:  hook.ModCtrl,
		42:  hook.ModShift,
		54:  hook.ModShift,
		56:  hook.ModAlt,
		100: hook.ModAlt,
		125: hook.ModSuper,
		126: hook.ModSuper,
	}
	return m
}

func (h *Hook) Start() error {
	h.stopped.Store(false)

	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return fmt.Errorf("failed to list input devices: %w", err)
	}

	for _, p := range paths {
		dev, err := evdev.Open(p.Path)
		if err != nil {
			logrus.Debugf("wayland hook: cannot open %s: %v", p.Path, err)
			continue
		}

		if !isKeyboard(dev) {
			dev.Close()
			continue
		}

		if err := dev.Grab(); err != nil {
			logrus.Errorf("wayland hook: failed to grab %s (%s): %v", p.Path, p.Name, err)
			dev.Close()
			continue
		}

		logrus.Infof("wayland hook: grabbed keyboard device %s (%s)", p.Path, p.Name)
		h.devices = append(h.devices, dev)
		h.wg.Add(1)
		go h.readLoop(dev)
	}

	if len(h.devices) == 0 {
		return fmt.Errorf("no keyboard devices found in /dev/input")
	}
	return nil
}

func isKeyboard(dev *evdev.InputDevice) bool {
	for _, t := range dev.CapableTypes() {
		if t == evdev.EV_KEY {
			for _, c := range dev.CapableEvents(evdev.EV_KEY) {
				if c == evdev.KEY_ENTER || c == evdev.KEY_SPACE || c == evdev.KEY_A {
					return true
				}
			}
		}
	}
	return false
}

func (h *Hook) readLoop(dev *evdev.InputDevice) {
	defer h.wg.Done()
	for {
		event, err := dev.ReadOne()
		if err != nil {
			if h.stopped.Load() {
				return
			}
			logrus.Errorf("wayland hook: read error from %s: %v", dev.Path(), err)
			return
		}
		h.processEvent(event)
	}
}

func (h *Hook) processEvent(event *evdev.InputEvent) {
	if event.Type != evdev.EV_KEY {
		return
	}

	code := uint16(event.Code)
	value := event.Value

	h.stateMu.Lock()

	if _, isMod := h.modMap[code]; isMod {
		if value == 1 || value == 2 {
			h.modState[code] = true
		} else {
			delete(h.modState, code)
		}

		if h.injector != nil {
			if err := h.injector.InjectEvent(code, int32(value)); err != nil {
				logrus.Warnf("wayland hook: re-inject modifier failed: %v", err)
			}
		}
		return
	}

	keyName, known := h.evdevToName[code]
	if !known {
		h.stateMu.Unlock()
		if h.injector != nil {
			if err := h.injector.InjectEvent(code, int32(value)); err != nil {
				logrus.Warnf("wayland hook: re-inject unknown key %d failed: %v", code, err)
			}
		}
		return
	}

	mods := h.currentModifiersLocked()

	if value == 0 {
		wasSuppressed := h.suppressed[code]
		delete(h.suppressed, code)
		h.stateMu.Unlock()

		if wasSuppressed {
			return
		}
		if h.handler != nil {
			h.handler(hook.KeyEvent{
				Type:      hook.KeyRelease,
				KeyName:   keyName,
				Modifiers: mods,
			})
		}
		if h.injector != nil {
			if err := h.injector.InjectEvent(code, int32(value)); err != nil {
				logrus.Warnf("wayland hook: re-inject release failed: %v", err)
			}
		}
		return
	}

	wasSuppressed := h.suppressed[code]
	h.stateMu.Unlock()

	if wasSuppressed && value == 2 {
		return
	}

	var intercepted bool
	ev := hook.KeyEvent{
		Type:      hook.KeyPress,
		KeyName:   keyName,
		Modifiers: mods,
	}
	if h.handler != nil {
		intercepted = h.handler(ev)
	}

	if intercepted {
		h.stateMu.Lock()
		h.suppressed[code] = true
		h.stateMu.Unlock()
	} else if h.injector != nil {
		if err := h.injector.InjectEvent(code, int32(value)); err != nil {
			logrus.Warnf("wayland hook: re-inject press failed: %v", err)
		}
	}
}

func (h *Hook) currentModifiersLocked() []hook.Modifier {
	seen := make(map[hook.Modifier]bool)
	mods := make([]hook.Modifier, 0, len(h.modState))
	for code := range h.modState {
		if mod, ok := h.modMap[code]; ok {
			if !seen[mod] {
				mods = append(mods, mod)
				seen[mod] = true
			}
		}
	}
	return mods
}

func (h *Hook) Stop() error {
	if !h.stopped.CompareAndSwap(false, true) {
		return nil
	}
	for _, dev := range h.devices {
		_ = dev.Ungrab()
		_ = dev.Close()
	}
	h.wg.Wait()
	h.devices = nil
	logrus.Info("wayland hook: stopped")
	return nil
}

func (h *Hook) UpdateGrabs(combos []hook.KeyCombo) error {
	h.combos = combos
	return nil
}

func (h *Hook) SetToggleGrab(combo hook.KeyCombo) error {
	h.toggleCombo = combo
	return nil
}

func (h *Hook) Pause() error {
	return nil
}

func (h *Hook) Resume() error {
	return nil
}
