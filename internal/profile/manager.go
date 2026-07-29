package profile

import (
	"regexp"
	"sync"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/engine"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/window"
	"github.com/sirupsen/logrus"
)

type ActiveRules struct {
	Profile *config.Profile
	Actions *engine.ActionEngine
}

type ProfileManager struct {
	profiles  []config.Profile
	defaultAR *ActiveRules
	active    *ActiveRules
	injector  inject.KeyInjector
	mu        sync.RWMutex
	onChanged func(current, defaultAR *ActiveRules)
}

func NewProfileManager(profiles []config.Profile, injector inject.KeyInjector) *ProfileManager {
	return &ProfileManager{
		profiles: profiles,
		injector: injector,
	}
}

func (pm *ProfileManager) Init() {
	var defaultProfile *config.Profile
	for i := range pm.profiles {
		if pm.profiles[i].IsDefault {
			defaultProfile = &pm.profiles[i]
			break
		}
	}
	if defaultProfile == nil && len(pm.profiles) > 0 {
		defaultProfile = &pm.profiles[0]
	}
	if defaultProfile != nil {
		pm.defaultAR = pm.buildActive(defaultProfile)
		pm.active = pm.defaultAR
	}
}

func (pm *ProfileManager) OnWindowFocus(win window.WindowInfo) {
	matched := pm.matchProfile(win)
	if matched == nil {
		matched = pm.defaultProfile()
		if matched == nil {
			return
		}
	}

	pm.mu.Lock()
	if pm.active != nil && pm.active.Profile.Name == matched.Name {
		pm.mu.Unlock()
		return
	}
	ar := pm.buildActive(matched)
	pm.active = ar
	cb := pm.onChanged
	pm.mu.Unlock()

	if cb != nil {
		cb(ar, pm.defaultAR)
	}
}

func (pm *ProfileManager) Active() *ActiveRules {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.active
}

func (pm *ProfileManager) Default() *ActiveRules {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.defaultAR
}

func (pm *ProfileManager) OnChanged(fn func(current, defaultAR *ActiveRules)) {
	pm.onChanged = fn
}

func (pm *ProfileManager) buildActive(p *config.Profile) *ActiveRules {
	return &ActiveRules{
		Profile: p,
		Actions: engine.NewActionEngine(p.Actions, pm.injector),
	}
}

func (pm *ProfileManager) defaultProfile() *config.Profile {
	for i := range pm.profiles {
		if pm.profiles[i].IsDefault {
			return &pm.profiles[i]
		}
	}
	return nil
}

func (pm *ProfileManager) matchProfile(win window.WindowInfo) *config.Profile {
	for i := range pm.profiles {
		if pm.profiles[i].IsDefault {
			continue
		}
		for _, m := range pm.profiles[i].Match {
			if matchEntry(m, win) {
				return &pm.profiles[i]
			}
		}
	}
	return nil
}

func matchEntry(m config.AppMatcher, win window.WindowInfo) bool {
	if m.AppID != "" {
		if re, err := regexp.Compile(m.AppID); err == nil {
			if re.MatchString(win.AppID) {
				return true
			}
		} else {
			logrus.Warnf("profile: invalid app_id regex %q: %v", m.AppID, err)
		}
	}
	if m.ProcessName != "" {
		if re, err := regexp.Compile(m.ProcessName); err == nil {
			if re.MatchString(win.ProcessName) {
				return true
			}
		} else {
			logrus.Warnf("profile: invalid process_name regex %q: %v", m.ProcessName, err)
		}
	}
	if m.Title != "" {
		if re, err := regexp.Compile(m.Title); err == nil {
			if re.MatchString(win.Title) {
				return true
			}
		} else {
			logrus.Warnf("profile: invalid title regex %q: %v", m.Title, err)
		}
	}
	return false
}
