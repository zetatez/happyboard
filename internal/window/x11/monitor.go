//go:build linux && cgo

package x11

//#cgo LDFLAGS: -lX11 -lXext
//#include <X11/Xlib.h>
//#include <X11/Xutil.h>
//#include <X11/Xatom.h>
import "C"

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/happyboard/happyboard/internal/window"
)

const focusPollInterval = 300 * time.Millisecond

type Monitor struct {
	display *C.Display
	stopCh  chan struct{}
	wg      sync.WaitGroup
	current window.WindowInfo
	mu      sync.Mutex
}

func NewMonitor() (*Monitor, error) {
	d := C.XOpenDisplay(nil)
	if d == nil {
		return nil, fmt.Errorf("cannot open X display")
	}
	return &Monitor{display: d, stopCh: make(chan struct{})}, nil
}

func (m *Monitor) Start(onChange func(window.WindowInfo)) error {
	m.wg.Add(1)
	go m.pollLoop(onChange)
	return nil
}

func (m *Monitor) Stop() error {
	close(m.stopCh)
	m.wg.Wait()
	if m.display != nil {
		C.XCloseDisplay(m.display)
		m.display = nil
	}
	return nil
}

func (m *Monitor) pollLoop(onChange func(window.WindowInfo)) {
	defer m.wg.Done()
	ticker := time.NewTicker(focusPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			info, err := m.getFocusedWindowInfo()
			if err != nil {
				continue
			}
			m.mu.Lock()
			changed := info.WindowID != m.current.WindowID
			m.mu.Unlock()
			if changed {
				m.mu.Lock()
				m.current = info
				m.mu.Unlock()
				onChange(info)
			}
		}
	}
}

func (m *Monitor) getFocusedWindowInfo() (window.WindowInfo, error) {
	var focus C.Window
	var revert int32
	C.XGetInputFocus(m.display, &focus, (*C.int)(unsafe.Pointer(&revert)))
	if focus == 0 || focus == 1 {
		return window.WindowInfo{}, nil
	}

	info := window.WindowInfo{WindowID: int64(focus)}
	info.Title = m.getWindowProperty(focus, "_NET_WM_NAME")
	if info.Title == "" {
		var namePtr *C.char
		C.XFetchName(m.display, focus, &namePtr)
		if namePtr != nil {
			info.Title = C.GoString(namePtr)
			C.XFree(unsafe.Pointer(namePtr))
		}
	}

	cls := m.getWMClass(focus)
	if cls != "" {
		info.AppID = cls
	}

	pid := m.getWindowProperty(focus, "_NET_WM_PID")
	if pid != "" {
		if p, err := strconv.Atoi(pid); err == nil {
			info.PID = p
			info.ProcessName = m.getProcessName(p)
		}
	}

	return info, nil
}

func (m *Monitor) getWindowProperty(win C.Window, name string) string {
	atom := C.XInternAtom(m.display, C.CString(name), C.False)
	if atom == 0 {
		return ""
	}

	var actualType C.Atom
	var actualFormat int32
	var nItems, bytesAfter C.ulong
	var propPtr *C.uchar

	status := C.XGetWindowProperty(
		m.display, win, atom, 0, 1024, C.False,
		C.AnyPropertyType, &actualType, (*C.int)(unsafe.Pointer(&actualFormat)),
		&nItems, &bytesAfter, &propPtr,
	)
	if status != 0 || propPtr == nil {
		return ""
	}
	defer C.XFree(unsafe.Pointer(propPtr))

	if actualFormat == 8 {
		return C.GoStringN((*C.char)(unsafe.Pointer(propPtr)), C.int(nItems))
	}
	return ""
}

func (m *Monitor) getWMClass(win C.Window) string {
	var hint C.XClassHint
	if C.XGetClassHint(m.display, win, &hint) == 0 {
		return ""
	}
	defer C.XFree(unsafe.Pointer(hint.res_name))
	defer C.XFree(unsafe.Pointer(hint.res_class))

	resClass := C.GoString(hint.res_class)
	resName := C.GoString(hint.res_name)
	if resClass != "" {
		return resClass
	}
	return resName
}

func (m *Monitor) getProcessName(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
