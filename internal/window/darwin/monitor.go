//go:build darwin && cgo

package darwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
#include <libproc.h>
#include <sys/types.h>

struct HBAppInfo {
	char  *bundleID;
	char  *name;
	char  *procName;
	pid_t  pid;
};

static struct HBAppInfo hbFrontmostApp(void) {
	struct HBAppInfo info = {NULL, NULL, NULL, 0};
	@autoreleasepool {
		NSWorkspace *ws = [NSWorkspace sharedWorkspace];
		NSRunningApplication *app = [ws frontmostApplication];
		if (app == nil) return info;

		NSString *bid = [app bundleIdentifier];
		NSString *nm  = [app localizedName];
		pid_t pid = [app processIdentifier];

		if (bid != nil) info.bundleID = strdup([bid UTF8String]);
		if (nm  != nil) info.name     = strdup([nm  UTF8String]);
		info.pid = pid;

		if (pid > 0) {
			char buf[256];
			if (proc_name(pid, buf, sizeof(buf)) > 0) {
				info.procName = strdup(buf);
			}
		}
	}
	return info;
}
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/happyboard/happyboard/internal/window"
)

type Monitor struct {
	mu       sync.Mutex
	stopped  atomic.Bool
	stopCh   chan struct{}
	current  window.WindowInfo
	interval time.Duration
}

func New() *Monitor {
	return &Monitor{interval: 300 * time.Millisecond}
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
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			win := m.detect()
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

func (m *Monitor) detect() window.WindowInfo {
	info := C.hbFrontmostApp()
	defer func() {
		if info.bundleID != nil {
			C.free(unsafe.Pointer(info.bundleID))
		}
		if info.name != nil {
			C.free(unsafe.Pointer(info.name))
		}
		if info.procName != nil {
			C.free(unsafe.Pointer(info.procName))
		}
	}()

	win := window.WindowInfo{
		PID: int(info.pid),
	}
	if info.bundleID != nil {
		win.AppID = C.GoString(info.bundleID)
	}
	if info.name != nil {
		win.Title = C.GoString(info.name)
	}
	if info.procName != nil {
		win.ProcessName = C.GoString(info.procName)
	}
	return win
}

func windowInfoEqual(a, b window.WindowInfo) bool {
	return a.Title == b.Title &&
		a.AppID == b.AppID &&
		a.ProcessName == b.ProcessName &&
		a.PID == b.PID
}
