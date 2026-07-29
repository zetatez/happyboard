package keydef

import (
	"fmt"
	"sort"
	"strings"

	"github.com/happyboard/happyboard/internal/hook"
)

type KeyMapping struct {
	Name      string
	X11Sym    int
	EvdevCode uint16
	MacCode   int
	WinVK     uint16
}

var keyMap = map[string]KeyMapping{
	"a": {"a", 0x0061, 30, 0, 0x41},
	"b": {"b", 0x0062, 48, 11, 0x42},
	"c": {"c", 0x0063, 46, 8, 0x43},
	"d": {"d", 0x0064, 32, 2, 0x44},
	"e": {"e", 0x0065, 18, 14, 0x45},
	"f": {"f", 0x0066, 33, 3, 0x46},
	"g": {"g", 0x0067, 34, 5, 0x47},
	"h": {"h", 0x0068, 35, 4, 0x48},
	"i": {"i", 0x0069, 23, 34, 0x49},
	"j": {"j", 0x006a, 36, 38, 0x4a},
	"k": {"k", 0x006b, 37, 40, 0x4b},
	"l": {"l", 0x006c, 38, 37, 0x4c},
	"m": {"m", 0x006d, 50, 46, 0x4d},
	"n": {"n", 0x006e, 49, 45, 0x4e},
	"o": {"o", 0x006f, 24, 31, 0x4f},
	"p": {"p", 0x0070, 25, 35, 0x50},
	"q": {"q", 0x0071, 16, 12, 0x51},
	"r": {"r", 0x0072, 19, 15, 0x52},
	"s": {"s", 0x0073, 31, 1, 0x53},
	"t": {"t", 0x0074, 20, 17, 0x54},
	"u": {"u", 0x0075, 22, 32, 0x55},
	"v": {"v", 0x0076, 47, 9, 0x56},
	"w": {"w", 0x0077, 17, 13, 0x57},
	"x": {"x", 0x0078, 45, 7, 0x58},
	"y": {"y", 0x0079, 21, 16, 0x59},
	"z": {"z", 0x007a, 44, 6, 0x5a},

	"0": {"0", 0x0030, 11, 29, 0x30},
	"1": {"1", 0x0031, 2, 18, 0x31},
	"2": {"2", 0x0032, 3, 19, 0x32},
	"3": {"3", 0x0033, 4, 20, 0x33},
	"4": {"4", 0x0034, 5, 21, 0x34},
	"5": {"5", 0x0035, 6, 23, 0x35},
	"6": {"6", 0x0036, 7, 22, 0x36},
	"7": {"7", 0x0037, 8, 26, 0x37},
	"8": {"8", 0x0038, 9, 28, 0x38},
	"9": {"9", 0x0039, 10, 25, 0x39},

	"f1":  {"f1", 0xffbe, 59, 122, 0x70},
	"f2":  {"f2", 0xffbf, 60, 120, 0x71},
	"f3":  {"f3", 0xffc0, 61, 99, 0x72},
	"f4":  {"f4", 0xffc1, 62, 118, 0x73},
	"f5":  {"f5", 0xffc2, 63, 96, 0x74},
	"f6":  {"f6", 0xffc3, 64, 97, 0x75},
	"f7":  {"f7", 0xffc4, 65, 98, 0x76},
	"f8":  {"f8", 0xffc5, 66, 100, 0x77},
	"f9":  {"f9", 0xffc6, 67, 101, 0x78},
	"f10": {"f10", 0xffc7, 68, 109, 0x79},
	"f11": {"f11", 0xffc8, 87, 103, 0x7a},
	"f12": {"f12", 0xffc9, 88, 111, 0x7b},

	"space":     {"space", 0x0020, 57, 49, 0x20},
	"enter":     {"enter", 0xff0d, 28, 36, 0x0d},
	"tab":       {"tab", 0xff09, 15, 48, 0x09},
	"esc":       {"esc", 0xff1b, 1, 53, 0x1b},
	"backspace": {"backspace", 0xff08, 14, 51, 0x08},
	"delete":    {"delete", 0xffff, 111, 117, 0x2e},
	"insert":    {"insert", 0xff63, 110, 114, 0x2d},
	"home":      {"home", 0xff50, 102, 115, 0x24},
	"end":       {"end", 0xff57, 107, 119, 0x23},
	"page_up":   {"page_up", 0xff55, 104, 116, 0x21},
	"page_down": {"page_down", 0xff56, 109, 121, 0x22},

	"left":  {"left", 0xff51, 105, 123, 0x25},
	"right": {"right", 0xff53, 106, 124, 0x27},
	"up":    {"up", 0xff52, 103, 126, 0x26},
	"down":  {"down", 0xff54, 108, 125, 0x28},

	"capslock":    {"capslock", 0xffe5, 58, 57, 0x14},
	"print":       {"print", 0xff61, 210, -1, 0x2c},
	"scroll_lock": {"scroll_lock", 0xff14, 70, -1, 0x91},
	"num_lock":    {"num_lock", 0xff7f, 69, -1, 0x90},

	"ctrl":  {"ctrl", 0xffe3, 29, 59, 0x11},
	"shift": {"shift", 0xffe1, 42, 56, 0x10},
	"alt":   {"alt", 0xffe9, 56, 58, 0x12},
	"super": {"super", 0xffeb, 125, 55, 0x5b},
	"hyper": {"hyper", 0xffed, 0, 0, 0},

	"minus":         {"minus", 0x002d, 12, 27, 0xbd},
	"equal":         {"equal", 0x003d, 13, 24, 0xbb},
	"left_bracket":  {"left_bracket", 0x005b, 26, 33, 0xdb},
	"right_bracket": {"right_bracket", 0x005d, 27, 30, 0xdd},
	"backslash":     {"backslash", 0x005c, 43, 42, 0xdc},
	"semicolon":     {"semicolon", 0x003b, 39, 41, 0xba},
	"quote":         {"quote", 0x0027, 40, 39, 0xde},
	"backtick":      {"backtick", 0x0060, 41, 50, 0xc0},
	"comma":         {"comma", 0x002c, 51, 43, 0xbc},
	"period":        {"period", 0x002e, 52, 47, 0xbe},
	"slash":         {"slash", 0x002f, 53, 44, 0xbf},
}

func GetKeyMap() map[string]KeyMapping {
	return keyMap
}

var modOrder = map[hook.Modifier]int{
	hook.ModCtrl:  0,
	hook.ModShift: 1,
	hook.ModAlt:   2,
	hook.ModSuper: 3,
	hook.ModHyper: 4,
}

func Normalize(name string) (string, error) {
	n := strings.ToLower(name)
	if _, ok := keyMap[n]; !ok {
		return "", fmt.Errorf("unknown key name: %q", name)
	}
	return n, nil
}

func NormalizeCombo(combo hook.KeyCombo) (hook.KeyCombo, error) {
	key, err := Normalize(combo.Key)
	if err != nil {
		return hook.KeyCombo{}, err
	}
	mods := make([]hook.Modifier, len(combo.Modifiers))
	copy(mods, combo.Modifiers)
	sort.SliceStable(mods, func(i, j int) bool {
		return modOrder[mods[i]] < modOrder[mods[j]]
	})
	return hook.KeyCombo{Modifiers: mods, Key: key}, nil
}

func ComboKey(combo hook.KeyCombo) string {
	mods := make([]hook.Modifier, len(combo.Modifiers))
	copy(mods, combo.Modifiers)
	sort.SliceStable(mods, func(i, j int) bool {
		return modOrder[mods[i]] < modOrder[mods[j]]
	})
	parts := make([]string, 0, len(mods)+1)
	for _, m := range mods {
		parts = append(parts, string(m))
	}
	parts = append(parts, strings.ToLower(combo.Key))
	return strings.Join(parts, "+")
}
