//go:build linux && cgo

package x11

//#cgo LDFLAGS: -lX11 -lXtst -lXext
//#include <X11/Xlib.h>
//#include <X11/Xutil.h>
//#include <X11/keysym.h>
//static int xevent_type(XEvent *ev) { return ev->xany.type; }
//
//static int _x_err_code = 0;
//static int _x_err_req = 0;
//static int x_error_handler(Display *d, XErrorEvent *e) {
//    _x_err_code = e->error_code;
//    _x_err_req = e->request_code;
//    return 0;
//}
//static void install_error_handler(Display *d) {
//    XSetErrorHandler(x_error_handler);
//    _x_err_code = 0;
//}
//static int check_error(Display *d) {
//    XSync(d, False);
//    int c = _x_err_code;
//    _x_err_code = 0;
//    return c;
//}
//static void allow_async(Display *d) { XAllowEvents(d, AsyncKeyboard, CurrentTime); }
//static void allow_replay(Display *d) { XAllowEvents(d, ReplayKeyboard, CurrentTime); }
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	log "github.com/sirupsen/logrus"
)

const (
	x11KeyPress   = 2
	x11KeyRelease = 3
	x11ShiftMask  = 1 << 0
	x11LockMask   = 1 << 1
	x11CtrlMask   = 1 << 2
	x11Mod1Mask   = 1 << 3
	x11Mod2Mask   = 1 << 4
	x11Mod3Mask   = 1 << 5
	x11Mod4Mask   = 1 << 6
)

type grabEntry struct {
	keycode C.KeyCode
	modmask uint
	combo   hook.KeyCombo
}

type Hook struct {
	display      *C.Display
	root         C.Window
	handler      func(hook.KeyEvent) bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	toggleGrab   *grabEntry
	mappingGrabs []grabEntry
	paused       bool
	symToName    map[int]string
}

func NewHook() (*Hook, error) {
	d := C.XOpenDisplay(nil)
	if d == nil {
		return nil, fmt.Errorf("cannot open X display")
	}
	C.install_error_handler(d)
	root := C.XDefaultRootWindow(d)
	h := &Hook{
		display:   d,
		root:      root,
		stopCh:    make(chan struct{}),
		symToName: buildSymToName(),
	}
	return h, nil
}

func buildSymToName() map[int]string {
	m := make(map[int]string, 150)
	for name, km := range keydef.GetKeyMap() {
		m[km.X11Sym] = name
	}
	return m
}

func (h *Hook) Start() error {
	h.wg.Add(1)
	go h.eventLoop()
	return nil
}

func (h *Hook) SetHandler(handler func(hook.KeyEvent) bool) {
	h.handler = handler
}

func (h *Hook) Stop() error {
	close(h.stopCh)
	h.releaseAllGrabs()
	h.wg.Wait()
	if h.display != nil {
		C.XCloseDisplay(h.display)
		h.display = nil
	}
	return nil
}

func (h *Hook) SetToggleGrab(combo hook.KeyCombo) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.toggleGrab != nil {
		h.ungrabEntry(h.toggleGrab)
	}
	entry, err := h.makeGrabEntry(combo)
	if err != nil {
		return err
	}
	h.grabEntry(entry)
	h.toggleGrab = entry
	log.Debugf("toggle grab set: %s", keydef.ComboKey(combo))
	return nil
}

func (h *Hook) UpdateGrabs(combos []hook.KeyCombo) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.mappingGrabs {
		h.ungrabEntry(&h.mappingGrabs[i])
	}
	h.mappingGrabs = nil
	for _, combo := range combos {
		entry, err := h.makeGrabEntry(combo)
		if err != nil {
			log.Warnf("failed to grab %s: %v", keydef.ComboKey(combo), err)
			continue
		}
		if h.grabEntry(entry) {
			h.mappingGrabs = append(h.mappingGrabs, *entry)
		}
	}
	C.XFlush(h.display)
	return nil
}

func (h *Hook) Pause() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.paused {
		return nil
	}
	h.paused = true
	for i := range h.mappingGrabs {
		h.ungrabEntry(&h.mappingGrabs[i])
	}
	C.XFlush(h.display)
	log.Info("keyboard hook paused")
	return nil
}

func (h *Hook) Resume() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.paused {
		return nil
	}
	h.paused = false
	for i := range h.mappingGrabs {
		h.grabEntry(&h.mappingGrabs[i])
	}
	C.XFlush(h.display)
	log.Info("keyboard hook resumed")
	return nil
}

func (h *Hook) makeGrabEntry(combo hook.KeyCombo) (*grabEntry, error) {
	sym, err := keydef.X11KeySym(combo.Key)
	if err != nil {
		return nil, err
	}
	code := C.XKeysymToKeycode(h.display, C.KeySym(sym))
	if code == 0 {
		return nil, fmt.Errorf("no keycode for key %q", combo.Key)
	}
	modmask := keydef.X11ModifierMask(combo.Modifiers)
	return &grabEntry{keycode: code, modmask: modmask, combo: combo}, nil
}

func (h *Hook) grabEntry(e *grabEntry) bool {
	masks := h.modVariants(e.modmask)
	successCount := 0
	for _, m := range masks {
		C.XGrabKey(h.display, C.int(e.keycode), C.uint(m),
			h.root, C.False, C.GrabModeAsync, C.GrabModeSync)
		errCode := int(C.check_error(h.display))
		if errCode != 0 {
			log.Debugf("XGrabKey variant 0x%x for %s failed (code %d), skipping variant", m, keydef.ComboKey(e.combo), errCode)
		} else {
			successCount++
		}
	}
	if successCount == 0 {
		log.Warnf("XGrabKey failed for %s - all variants failed, key may be grabbed by another app (dwm/terminal?)",
			keydef.ComboKey(e.combo))
		return false
	}
	return true
}

func (h *Hook) ungrabEntry(e *grabEntry) {
	masks := h.modVariants(e.modmask)
	for _, m := range masks {
		C.XUngrabKey(h.display, C.int(e.keycode), C.uint(m), h.root)
	}
}

func (h *Hook) modVariants(base uint) []uint {
	if base == 0 {
		return []uint{0, x11LockMask, x11Mod2Mask, x11LockMask | x11Mod2Mask}
	}
	return []uint{
		base,
		base | x11LockMask,
		base | x11Mod2Mask,
		base | x11LockMask | x11Mod2Mask,
	}
}

func (h *Hook) releaseAllGrabs() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.mappingGrabs {
		h.ungrabEntry(&h.mappingGrabs[i])
	}
	h.mappingGrabs = nil
	if h.toggleGrab != nil {
		h.ungrabEntry(h.toggleGrab)
		h.toggleGrab = nil
	}
	C.XFlush(h.display)
}

func (h *Hook) eventLoop() {
	defer h.wg.Done()
	for {
		select {
		case <-h.stopCh:
			return
		default:
		}

		if C.XPending(h.display) == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		for C.XPending(h.display) > 0 {
			var ev C.XEvent
			C.XNextEvent(h.display, &ev)
			evType := int(C.xevent_type(&ev))

			if evType != x11KeyPress && evType != x11KeyRelease {
				C.allow_async(h.display)
				C.XFlush(h.display)
				continue
			}

			ke := (*C.XKeyEvent)(unsafe.Pointer(&ev))
			keycode := int(ke.keycode)
			state := uint(ke.state)

			keyName := h.keyCodeToName(keycode)
			if keyName == "" {
				C.allow_replay(h.display)
				C.XFlush(h.display)
				continue
			}

			mods := h.stateToMods(state)
			evType2 := hook.KeyPress
			if evType == x11KeyRelease {
				evType2 = hook.KeyRelease
			}

			keyEvent := hook.KeyEvent{
				Type:      evType2,
				KeyName:   keyName,
				Modifiers: mods,
			}

			log.Debugf("x11 event: type=%d key=%s mods=%v", evType2, keyName, mods)

			intercepted := false
			if h.handler != nil {
				intercepted = h.handler(keyEvent)
			}

			if intercepted {
				log.Debugf("x11: event intercepted, discarding original")
				C.allow_async(h.display)
			} else {
				C.allow_replay(h.display)
			}
			C.XFlush(h.display)
		}
	}
}

func (h *Hook) keyCodeToName(code int) string {
	sym := int(C.XKeycodeToKeysym(h.display, C.KeyCode(code), 0))
	if sym == 0 {
		return ""
	}
	if name, ok := h.symToName[sym]; ok {
		return name
	}
	return ""
}

func (h *Hook) stateToMods(state uint) []hook.Modifier {
	var mods []hook.Modifier
	if state&x11CtrlMask != 0 {
		mods = append(mods, hook.ModCtrl)
	}
	if state&x11ShiftMask != 0 {
		mods = append(mods, hook.ModShift)
	}
	if state&x11Mod1Mask != 0 {
		mods = append(mods, hook.ModAlt)
	}
	if state&x11Mod4Mask != 0 {
		mods = append(mods, hook.ModSuper)
	}
	if state&x11Mod3Mask != 0 {
		mods = append(mods, hook.ModHyper)
	}
	return mods
}
