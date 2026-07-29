package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/happyboard/happyboard/internal/config"
	"github.com/happyboard/happyboard/internal/dispatch"
	"github.com/happyboard/happyboard/internal/hook"
	"github.com/happyboard/happyboard/internal/inject"
	"github.com/happyboard/happyboard/internal/profile"
	"github.com/happyboard/happyboard/internal/window"
	log "github.com/sirupsen/logrus"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()
}

func main() {
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config from %s: %v\n", configPath, err)
		os.Exit(1)
	}

	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config validation failed: %v\n", err)
		os.Exit(1)
	}

	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	log.Infof("happyboard starting (platform=%s, config=%s, hijack_enabled=%v)",
		runtime.GOOS, configPath, cfg.HijackEnabled)

	hk, inj, mon, err := createPlatformComponents(cfg)
	if err != nil {
		log.Fatalf("failed to create platform components: %v", err)
	}
	defer hk.Stop()
	defer inj.Close()
	defer mon.Stop()

	pm := profile.NewProfileManager(cfg.Profiles, inj)
	pm.Init()

	dispatcher := dispatch.New(hk, pm)
	dispatcher.Init(cfg.HijackEnabled, cfg.ToggleKey)

	var currentCombos []hook.KeyCombo
	if active := pm.Active(); active != nil {
		currentCombos = extractCombos(active)
	}
	if def := pm.Default(); def != nil && def != pm.Active() {
		currentCombos = append(currentCombos, extractCombos(def)...)
	}

	if enabled := dispatcher.IsEnabled(); enabled && len(currentCombos) > 0 {
		if err := hk.UpdateGrabs(currentCombos); err != nil {
			log.Errorf("failed to set initial grabs: %v", err)
		}
	}

	if err := hk.SetToggleGrab(cfg.ToggleKey); err != nil {
		log.Warnf("failed to set toggle grab: %v", err)
	}

	pm.OnChanged(func(current, defaultAR *profile.ActiveRules) {
		log.Infof("profile switched to: %s", current.Profile.Name)
		combos := extractCombos(current)
		if defaultAR != nil && defaultAR != current {
			combos = append(combos, extractCombos(defaultAR)...)
		}
		if dispatcher.IsEnabled() {
			if err := hk.UpdateGrabs(combos); err != nil {
				log.Errorf("failed to update grabs on profile switch: %v", err)
			}
		}
	})

	if err := mon.Start(func(win window.WindowInfo) {
		log.Debugf("window focus changed: app_id=%s title=%s", win.AppID, win.Title)
		pm.OnWindowFocus(win)
	}); err != nil {
		log.Fatalf("failed to start window monitor: %v", err)
	}

	if err := hk.Start(); err != nil {
		log.Fatalf("failed to start keyboard hook: %v", err)
	}

	setHookHandler(hk, dispatcher)

	log.Info("happyboard is running. Press Ctrl+C to exit.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down...")
}

func extractCombos(ar *profile.ActiveRules) []hook.KeyCombo {
	var combos []hook.KeyCombo
	seen := make(map[string]bool)

	addCombo := func(c hook.KeyCombo) {
		key := fmt.Sprintf("%v+%s", c.Modifiers, c.Key)
		if !seen[key] {
			seen[key] = true
			combos = append(combos, c)
		}
	}

	for _, a := range ar.Profile.Actions {
		addCombo(a.Trigger)
	}

	return combos
}

func createPlatformComponents(cfg *config.Config) (hook.KeyboardHook, inject.KeyInjector, window.WindowFocusMonitor, error) {
	switch runtime.GOOS {
	case "linux":
		return createLinuxComponents(cfg)
	case "darwin":
		return createDarwinComponents()
	case "windows":
		return createWindowsComponents()
	default:
		return nil, nil, nil, fmt.Errorf("platform %s not yet supported", runtime.GOOS)
	}
}
