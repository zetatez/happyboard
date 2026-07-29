//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	WH_KEYBOARD_LL = 13
	HC_ACTION      = 0

	WM_KEYDOWN    = 0x0100
	WM_KEYUP      = 0x0101
	WM_SYSKEYDOWN = 0x0104
	WM_SYSKEYUP   = 0x0105
	WM_QUIT       = 0x0012

	LLKHF_INJECTED = 0x0010

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkLWin    = 0x5B
	vkRWin    = 0x5C
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      [2]int32
}

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	pPostThreadMessageW  = user32.NewProc("PostThreadMessageW")

	hookProc = syscall.NewCallback(keyboardHookProc)
)

var vkToName = func() map[uint32]string {
	m := make(map[uint32]string, len(keydef.GetKeyMap()))
	for name, km := range keydef.GetKeyMap() {
		if _, exists := m[uint32(km.WinVK)]; !exists {
			m[uint32(km.WinVK)] = name
		}
	}
	return m
}()

var (
	globalMu   sync.Mutex
	globalHook *Hook
)

type Hook struct {
	mu          sync.Mutex
	wg          sync.WaitGroup
	handler     func(hook.KeyEvent) bool
	combos      []hook.KeyCombo
	toggleCombo hook.KeyCombo
	paused      atomic.Bool
	stopped     atomic.Bool

	hookHandle uintptr
	threadID   uint32
}

func New() *Hook {
	return &Hook{}
}

func (h *Hook) Start() error {
	h.stopped.Store(false)

	installed := make(chan error, 1)
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		h.mu.Lock()
		h.threadID = windows.GetCurrentThreadId()
		h.mu.Unlock()

		var hmod windows.Handle
		if err := windows.GetModuleHandleEx(0, nil, &hmod); err != nil {
			log.Warnf("windows hook: GetModuleHandleEx failed: %v", err)
		}

		handle, _, callErr := pSetWindowsHookExW.Call(
			uintptr(WH_KEYBOARD_LL),
			hookProc,
			uintptr(hmod),
			0,
		)
		if handle == 0 {
			installed <- fmt.Errorf("SetWindowsHookExW failed: %w", callErr)
			return
		}

		h.mu.Lock()
		h.hookHandle = handle
		globalMu.Lock()
		globalHook = h
		globalMu.Unlock()
		h.mu.Unlock()

		installed <- nil
		h.messageLoop()

		pUnhookWindowsHookEx.Call(handle)
		globalMu.Lock()
		if globalHook == h {
			globalHook = nil
		}
		globalMu.Unlock()
		log.Info("windows hook: stopped")
	}()

	return <-installed
}

func (h *Hook) messageLoop() {
	for {
		var msg MSG
		ret, _, _ := pGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		r := int32(ret)
		if r == 0 || r < 0 {
			if r < 0 {
				log.Warnf("windows hook: GetMessage error")
			}
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (h *Hook) Stop() error {
	if !h.stopped.CompareAndSwap(false, true) {
		return nil
	}
	h.mu.Lock()
	tid := h.threadID
	h.mu.Unlock()
	if tid != 0 {
		_, _, _ = pPostThreadMessageW.Call(uintptr(tid), uintptr(WM_QUIT), 0, 0)
	}
	h.wg.Wait()
	return nil
}

func (h *Hook) UpdateGrabs(combos []hook.KeyCombo) error {
	h.mu.Lock()
	h.combos = combos
	h.mu.Unlock()
	return nil
}

func (h *Hook) SetToggleGrab(combo hook.KeyCombo) error {
	h.mu.Lock()
	h.toggleCombo = combo
	h.mu.Unlock()
	return nil
}

func (h *Hook) Pause() error {
	h.paused.Store(true)
	return nil
}

func (h *Hook) Resume() error {
	h.paused.Store(false)
	return nil
}

func (h *Hook) SetHandler(handler func(hook.KeyEvent) bool) {
	h.mu.Lock()
	h.handler = handler
	h.mu.Unlock()
}

func (h *Hook) Close() error {
	return nil
}

func keyboardHookProc(code int32, wParam uintptr, lParam uintptr) uintptr {
	if code != HC_ACTION {
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}

	globalMu.Lock()
	h := globalHook
	globalMu.Unlock()
	if h == nil {
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}

	kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))

	// Injected events (e.g. produced by SendInput) must pass through, otherwise
	// re-injected combos would re-enter the handler and loop forever.
	if kb.Flags&LLKHF_INJECTED != 0 {
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}

	var eventType hook.KeyEventType
	switch wParam {
	case WM_KEYDOWN, WM_SYSKEYDOWN:
		eventType = hook.KeyPress
	case WM_KEYUP, WM_SYSKEYUP:
		eventType = hook.KeyRelease
	default:
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}

	keyName, ok := vkToName[kb.VkCode]
	if !ok {
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}

	ev := hook.KeyEvent{
		Type:      eventType,
		KeyName:   keyName,
		Modifiers: currentModifiers(),
	}

	h.mu.Lock()
	handler := h.handler
	h.mu.Unlock()

	if handler != nil {
		if handler(ev) {
			return 1
		}
		r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return r
	}
	}
	r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return r
}

func currentModifiers() []hook.Modifier {
	var mods []hook.Modifier
	if keyDown(vkControl) {
		mods = append(mods, hook.ModCtrl)
	}
	if keyDown(vkShift) {
		mods = append(mods, hook.ModShift)
	}
	if keyDown(vkMenu) {
		mods = append(mods, hook.ModAlt)
	}
	if keyDown(vkLWin) || keyDown(vkRWin) {
		mods = append(mods, hook.ModSuper)
	}
	return mods
}

func keyDown(vk int32) bool {
	r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
	return int16(r) < 0
}
