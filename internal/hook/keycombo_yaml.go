package hook

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var modOrder = map[Modifier]int{
	ModCtrl:  0,
	ModShift: 1,
	ModAlt:   2,
	ModSuper: 3,
	ModHyper: 4,
}

var modAliases = map[string]Modifier{
	"ctrl":     ModCtrl,
	"control":  ModCtrl,
	"ctl":      ModCtrl,
	"shift":    ModShift,
	"alt":      ModAlt,
	"option":   ModAlt,
	"opt":      ModAlt,
	"super":    ModSuper,
	"cmd":      ModSuper,
	"command":  ModSuper,
	"meta":     ModSuper,
	"win":      ModSuper,
	"hyper":    ModHyper,
	"capslock": ModHyper,
}

func (c *KeyCombo) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		return c.parseString(value.Value)
	case yaml.MappingNode:
		var m struct {
			Modifiers []string `yaml:"modifiers"`
			Key       string   `yaml:"key"`
		}
		if err := value.Decode(&m); err != nil {
			return err
		}
		c.Key = strings.ToLower(m.Key)
		for _, s := range m.Modifiers {
			mod, err := lookupModifier(s)
			if err != nil {
				return err
			}
			c.Modifiers = append(c.Modifiers, mod)
		}
		c.sortModifiers()
		return nil
	default:
		return fmt.Errorf("expected string or map for key combo, got kind %d", value.Kind)
	}
}

func (c *KeyCombo) parseString(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty key combo")
	}
	parts := strings.Split(s, "+")
	key := strings.TrimSpace(parts[len(parts)-1])
	if key == "" {
		return fmt.Errorf("empty key in combo: %q", s)
	}
	c.Key = strings.ToLower(key)
	for _, p := range parts[:len(parts)-1] {
		mod, err := lookupModifier(strings.TrimSpace(p))
		if err != nil {
			return err
		}
		c.Modifiers = append(c.Modifiers, mod)
	}
	c.sortModifiers()
	return nil
}

func (c *KeyCombo) sortModifiers() {
	sort.SliceStable(c.Modifiers, func(i, j int) bool {
		return modOrder[c.Modifiers[i]] < modOrder[c.Modifiers[j]]
	})
}

func lookupModifier(s string) (Modifier, error) {
	mod, ok := modAliases[strings.ToLower(s)]
	if !ok {
		return "", fmt.Errorf("unknown modifier: %q", s)
	}
	return mod, nil
}
