package hook

type KeyEventType int

const (
	KeyPress KeyEventType = iota
	KeyRelease
)

type Modifier string

const (
	ModCtrl  Modifier = "ctrl"
	ModShift Modifier = "shift"
	ModAlt   Modifier = "alt"
	ModSuper Modifier = "super"
	ModHyper Modifier = "hyper"
)

type KeyCombo struct {
	Modifiers []Modifier
	Key       string
}

type KeyEvent struct {
	Type      KeyEventType
	KeyName   string
	Modifiers []Modifier
}

type KeyboardHook interface {
	Start() error
	Stop() error
	UpdateGrabs(combos []KeyCombo) error
	SetToggleGrab(combo KeyCombo) error
	Pause() error
	Resume() error
}
