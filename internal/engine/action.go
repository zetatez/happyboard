package engine

import (
	"time"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/keydef"
	"github.com/happyboard/happyboard/internal/script"
	"github.com/sirupsen/logrus"
)

type ActionEngine struct {
	actions    map[string]*config.ActionRule
	injector   inject.KeyInjector
	OnInternal func(command string)
}

func NewActionEngine(rules []config.ActionRule, injector inject.KeyInjector) *ActionEngine {
	a := &ActionEngine{
		actions:  make(map[string]*config.ActionRule),
		injector: injector,
	}
	for i := range rules {
		normalized, err := keydef.NormalizeCombo(rules[i].Trigger)
		if err != nil {
			logrus.Warnf("action engine: invalid trigger combo: %v", err)
			continue
		}
		key := keydef.ComboKey(normalized)
		a.actions[key] = &rules[i]
	}
	return a
}

func (a *ActionEngine) Match(ev hook.KeyEvent) *config.ActionRule {
	if ev.Type != hook.KeyPress {
		return nil
	}
	combo := hook.KeyCombo{Modifiers: ev.Modifiers, Key: ev.KeyName}
	normalized, err := keydef.NormalizeCombo(combo)
	if err != nil {
		return nil
	}
	return a.actions[keydef.ComboKey(normalized)]
}

func (a *ActionEngine) Execute(action *config.ActionRule) {
	switch action.Type {
	case config.ActionShell:
		if err := script.RunShell(action.Command, action.Async); err != nil {
			logrus.Errorf("action engine: shell command failed: %v", err)
		}
	case config.ActionInternal:
		if a.OnInternal != nil {
			a.OnInternal(action.Command)
		} else {
			logrus.Infof("action engine: internal action (no handler wired): %s", action.Command)
		}
	case config.ActionTextInput:
		go func() {
			time.Sleep(150 * time.Millisecond)
			if err := a.injector.TypeText(action.Text, action.Delay); err != nil {
				logrus.Errorf("action engine: text input failed: %v", err)
			}
			if action.Enter {
				if err := a.injector.TapKey("enter"); err != nil {
					logrus.Errorf("action engine: tap enter failed: %v", err)
				}
			}
		}()
	default:
		logrus.Warnf("action engine: unknown action type: %s", action.Type)
	}
}
