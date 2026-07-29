package dispatch

import (
	"sync"

	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/keydef"
	"github.com/happyboard/happyboard/internal/profile"
	"github.com/sirupsen/logrus"
)

type Dispatcher struct {
	mu           sync.Mutex
	enabled      bool
	toggleKey    hook.KeyCombo
	toggleKeyStr string
	hook         hook.KeyboardHook
	pm           *profile.ProfileManager
}

func New(hk hook.KeyboardHook, pm *profile.ProfileManager) *Dispatcher {
	return &Dispatcher{
		hook: hk,
		pm:   pm,
	}
}

func (d *Dispatcher) Init(enabled bool, toggleKey hook.KeyCombo) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = enabled
	d.toggleKey = toggleKey
	normalized, err := keydef.NormalizeCombo(toggleKey)
	if err != nil {
		logrus.Warnf("dispatcher: invalid toggle key, toggle disabled: %v", err)
		d.toggleKeyStr = ""
		return
	}
	d.toggleKeyStr = keydef.ComboKey(normalized)
}

func (d *Dispatcher) HandleEvent(ev hook.KeyEvent) bool {
	if d.toggleKeyStr != "" && ev.Type == hook.KeyPress {
		combo := hook.KeyCombo{Modifiers: ev.Modifiers, Key: ev.KeyName}
		if normalized, err := keydef.NormalizeCombo(combo); err == nil {
			if keydef.ComboKey(normalized) == d.toggleKeyStr {
				d.toggleHijack()
				return true
			}
		}
	}

	d.mu.Lock()
	enabled := d.enabled
	d.mu.Unlock()

	if !enabled {
		return false
	}

	if d.dispatch(ev, d.pm.Active()) {
		return true
	}

	if d.pm.Default() != nil && d.pm.Active() != d.pm.Default() {
		return d.dispatch(ev, d.pm.Default())
	}

	return false
}

func (d *Dispatcher) dispatch(ev hook.KeyEvent, rules *profile.ActiveRules) bool {
	if rules == nil {
		return false
	}

	if action := rules.Actions.Match(ev); action != nil {
		logrus.Debugf("dispatcher: action matched: %s (type=%s)", action.Trigger.Key, action.Type)
		rules.Actions.Execute(action)
		return true
	}

	return false
}

func (d *Dispatcher) toggleHijack() {
	d.mu.Lock()
	d.enabled = !d.enabled
	enabled := d.enabled
	d.mu.Unlock()

	if enabled {
		logrus.Info("dispatcher: hijack enabled")
		if err := d.hook.Resume(); err != nil {
			logrus.Errorf("dispatcher: hook resume failed: %v", err)
		}
	} else {
		logrus.Info("dispatcher: hijack disabled")
		if err := d.hook.Pause(); err != nil {
			logrus.Errorf("dispatcher: hook pause failed: %v", err)
		}
	}
}

func (d *Dispatcher) IsEnabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enabled
}
