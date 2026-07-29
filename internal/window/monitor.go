package window

type WindowInfo struct {
	WindowID    int64
	Title       string
	AppID       string
	ProcessName string
	PID         int
}

type WindowFocusMonitor interface {
	Start(onChange func(WindowInfo)) error
	Stop() error
}
