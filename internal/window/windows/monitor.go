//go:build windows

package windows

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/happyboard/happyboard/internal/window"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const focusPollInterval = 300 * time.Millisecond

var (
	pGetWindowTextW       = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")
	pGetWindowTextLengthW = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextLengthW")
)

type Monitor struct {
	mu      sync.Mutex
	stopped atomic.Bool
	stopCh  chan struct{}
	current window.WindowInfo
}

func New() *Monitor {
	return &Monitor{stopCh: make(chan struct{})}
}

func (m *Monitor) Start(onChange func(window.WindowInfo)) error {
	m.stopped.Store(false)
	m.stopCh = make(chan struct{})
	go m.pollLoop(onChange)
	return nil
}

func (m *Monitor) Stop() error {
	if !m.stopped.CompareAndSwap(false, true) {
		return nil
	}
	close(m.stopCh)
	return nil
}

func (m *Monitor) pollLoop(onChange func(window.WindowInfo)) {
	ticker := time.NewTicker(focusPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			win, err := m.detect()
			if err != nil {
				log.Debugf("windows monitor: detect error: %v", err)
				continue
			}
			m.mu.Lock()
			changed := !windowInfoEqual(m.current, win)
			if changed {
				m.current = win
			}
			m.mu.Unlock()
			if changed && onChange != nil {
				onChange(win)
			}
		}
	}
}

func (m *Monitor) detect() (window.WindowInfo, error) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return window.WindowInfo{}, nil
	}

	info := window.WindowInfo{WindowID: int64(hwnd)}
	info.Title = windowText(uintptr(hwnd))

	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		log.Debugf("windows monitor: GetWindowThreadProcessId failed: %v", err)
	}
	info.PID = int(pid)

	if pid != 0 {
		if exePath, err := processExePath(pid); err == nil && exePath != "" {
			base := filepath.Base(exePath)
			info.AppID = base
			info.ProcessName = strings.TrimSuffix(base, filepath.Ext(base))
		}
	}
	return info, nil
}

func windowText(hwnd uintptr) string {
	length, _, _ := pGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	n, _, _ := pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func processExePath(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func windowInfoEqual(a, b window.WindowInfo) bool {
	return a.WindowID == b.WindowID &&
		a.Title == b.Title &&
		a.AppID == b.AppID &&
		a.ProcessName == b.ProcessName &&
		a.PID == b.PID
}
