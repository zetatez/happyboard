//go:build linux

package wayland

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/happyboard/happyboard/internal/window"
	"github.com/sirupsen/logrus"
)

type Monitor struct {
	mu       sync.Mutex
	stopped  atomic.Bool
	stopCh   chan struct{}
	current  window.WindowInfo
	interval time.Duration
}

func New() *Monitor {
	return &Monitor{
		interval: 500 * time.Millisecond,
	}
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
			win, err := m.detect()
			if err != nil {
				logrus.Debugf("wayland monitor: detect error: %v", err)
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
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))

	switch {
	case strings.Contains(desktop, "hyprland"):
		return detectHyprland()
	case strings.Contains(desktop, "sway"):
		return detectSway()
	default:
		return window.WindowInfo{}, nil
	}
}

type hyprlandWindow struct {
	Class string `json:"class"`
	Title string `json:"title"`
	PID   int    `json:"pid"`
}

func detectHyprland() (window.WindowInfo, error) {
	cmd := exec.Command("hyprctl", "activewindow", "-j")
	output, err := cmd.Output()
	if err != nil {
		return window.WindowInfo{}, fmt.Errorf("hyprctl activewindow: %w", err)
	}
	var hw hyprlandWindow
	if err := json.Unmarshal(output, &hw); err != nil {
		return window.WindowInfo{}, fmt.Errorf("parse hyprctl output: %w", err)
	}
	return window.WindowInfo{
		Title:       hw.Title,
		AppID:       hw.Class,
		PID:         hw.PID,
		ProcessName: processNameFromPID(hw.PID),
	}, nil
}

type swayNode struct {
	Type             string           `json:"type"`
	Focused          bool             `json:"focused"`
	Name             string           `json:"name"`
	AppID            string           `json:"app_id"`
	PID              int              `json:"pid"`
	Nodes            []swayNode       `json:"nodes"`
	FloatingNodes    []swayNode       `json:"floating_nodes"`
	WindowProperties *swayWindowProps `json:"window_properties"`
}

type swayWindowProps struct {
	Class string `json:"class"`
}

func detectSway() (window.WindowInfo, error) {
	cmd := exec.Command("swaymsg", "-t", "get_tree")
	output, err := cmd.Output()
	if err != nil {
		return window.WindowInfo{}, fmt.Errorf("swaymsg get_tree: %w", err)
	}
	var root swayNode
	if err := json.Unmarshal(output, &root); err != nil {
		return window.WindowInfo{}, fmt.Errorf("parse swaymsg output: %w", err)
	}
	focused := findFocusedSwayNode(&root)
	if focused == nil {
		return window.WindowInfo{}, nil
	}
	appID := focused.AppID
	if appID == "" && focused.WindowProperties != nil {
		appID = focused.WindowProperties.Class
	}
	return window.WindowInfo{
		Title:       focused.Name,
		AppID:       appID,
		PID:         focused.PID,
		ProcessName: processNameFromPID(focused.PID),
	}, nil
}

func findFocusedSwayNode(node *swayNode) *swayNode {
	if node.Focused {
		return node
	}
	for i := range node.Nodes {
		if found := findFocusedSwayNode(&node.Nodes[i]); found != nil {
			return found
		}
	}
	for i := range node.FloatingNodes {
		if found := findFocusedSwayNode(&node.FloatingNodes[i]); found != nil {
			return found
		}
	}
	return nil
}

func processNameFromPID(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func windowInfoEqual(a, b window.WindowInfo) bool {
	return a.Title == b.Title &&
		a.AppID == b.AppID &&
		a.ProcessName == b.ProcessName &&
		a.PID == b.PID
}
