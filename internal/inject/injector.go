package inject

type KeyInjector interface {
	TapKey(keyName string) error
	TypeText(text string, delayMs int) error
	Close() error
}
