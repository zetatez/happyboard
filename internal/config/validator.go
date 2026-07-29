package config

import (
	"fmt"

	"github.com/happyboard/happyboard/internal/keydef"
)

func Validate(cfg *Config) error {
	hasDefault := false
	for _, p := range cfg.Profiles {
		if p.IsDefault {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		return fmt.Errorf("no default profile found; at least one profile must have is_default: true")
	}

	var toggleKeyStr string
	if cfg.ToggleKey.Key != "" {
		normalized, err := keydef.NormalizeCombo(cfg.ToggleKey)
		if err != nil {
			return fmt.Errorf("invalid toggle_key: %w", err)
		}
		cfg.ToggleKey = normalized
		toggleKeyStr = keydef.ComboKey(normalized)
	}

	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		seen := make(map[string]bool)

		for j := range p.Actions {
			a := &p.Actions[j]
			normalized, err := keydef.NormalizeCombo(a.Trigger)
			if err != nil {
				return fmt.Errorf("profile %q action[%d]: invalid trigger key: %w", p.Name, j, err)
			}
			a.Trigger = normalized
			comboStr := keydef.ComboKey(normalized)

			if seen[comboStr] {
				return fmt.Errorf("profile %q: duplicate key combo %s", p.Name, comboStr)
			}
			seen[comboStr] = true

			if toggleKeyStr != "" && comboStr == toggleKeyStr {
				return fmt.Errorf("profile %q: key combo %s conflicts with toggle_key", p.Name, comboStr)
			}

			switch a.Type {
			case ActionTextInput:
				if a.Text == "" {
					return fmt.Errorf("profile %q action[%d]: text_input requires text field", p.Name, j)
				}
			case ActionShell, ActionInternal:
				if a.Command == "" {
					return fmt.Errorf("profile %q action[%d]: %s action requires command field", p.Name, j, a.Type)
				}
			default:
				return fmt.Errorf("profile %q action[%d]: unknown action type %q", p.Name, j, a.Type)
			}
		}
	}

	return nil
}
