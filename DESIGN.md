# HappyBoard - 跨平台键盘控制器设计稿

## 1. 概述

基于 Go 的跨平台桌面键盘控制器，支持 Linux、macOS、Windows。

**核心功能**：

1. **热键动作** - 按键组合触发 shell 命令 / 内置动作 / 文本输入
2. **应用感知 Profile** - 检测聚焦窗口，自动切换动作方案
3. **劫持开关** - 可配置默认开关状态，支持运行时 toggle 按键切换

> 说明：按键映射（from->to 重映射）因软件层面无法完美实现（修饰键状态、抢占竞争、平台差异等），已从设计中移除。保留按键触发 shell 命令、内置动作、文本输入三类动作。

**平台支持**：

 | 平台            | 后端                       | CGO    |
 | ------          | ------                     | -----  |
 | Linux / X11     | XGrabKey + XRecord + XTest | 需要   |
 | Linux / Wayland | evdev + uinput             | 不需要 |
 | macOS           | CGEventTap + CGEventPost   | 需要   |
 | Windows         | WH_KEYBOARD_LL + SendInput | 不需要 |

> Linux 启动时通过 `XDG_SESSION_TYPE` 自动检测 X11/Wayland，也可配置强制指定。

---

## 2. 平台抽象层

### 2.1 设计原则

接口驱动 + 编译时平台选择。三组平台无关接口，各平台独立实现，通过 Go build tags 选择。

**平台无关层**（共享）：
- ConfigManager / EventDispatcher / ProfileManager
- ActionEngine (shell/internal/text)
- ScriptRunner / keydef (统一按键名)

**平台接口**：
- `KeyboardHook` - 拦截/监听键盘事件
- `KeyInjector` - 注入合成按键
- `WindowFocusMonitor` - 监控窗口聚焦

### 2.2 核心接口

```go
type KeyboardHook interface {
    Start(onEvent func(KeyEvent)) error
    Stop() error
    UpdateGrabs(combos []KeyCombo) error   // 更新动作触发键 grab（Profile 切换时调用）
    SetToggleGrab(combo KeyCombo) error    // 设置 toggle 按键（始终 grab，不受 Pause/Resume 影响）
    Pause() error                          // 释放动作 grab，保留 toggle key grab
    Resume() error                         // 恢复动作 grab
}

type KeyInjector interface {
    TapKey(keyName string) error
    PressKey(keyName string) error
    ReleaseKey(keyName string) error
    TypeText(text string, delayMs int) error
}

type WindowFocusMonitor interface {
    Start(onChange func(WindowInfo)) error
    Stop() error
    GetCurrent() (WindowInfo, error)
}
```

### 2.3 统一数据结构

```go
type KeyEvent struct {
    Type        KeyEventType // KeyPress | KeyRelease
    KeyName     string       // 统一按键名，如 "a", "f1", "left"
    Modifiers   []Modifier
    Intercepted bool
    Timestamp   time.Time
}

type WindowInfo struct {
    Title       string
    AppID       string // 平台特定：Linux=WM_CLASS, macOS=Bundle ID, Windows=可执行文件名
    ProcessName string
    PID         int
}
```

### 2.4 AppID 平台映射

配置中 `app_id` 为正则表达式，匹配当前平台 AppID：

 | 平台          | AppID 来源                | 示例                          |
 | ------        | -----------               | ------                        |
 | Linux/X11     | WM_CLASS                  | `"Code"`, `"firefox.Firefox"` |
 | Linux/Wayland | app_id（wlroots）/ 进程名 | `"code"`, `"firefox"`         |
 | macOS         | Bundle Identifier         | `"com.microsoft.VSCode"`      |
 | Windows       | 进程可执行文件名          | `"Code.exe"`                  |

---

## 3. 系统架构

```
KeyboardHook (平台实现)
  │ key events
  ▼
EventDispatcher ──► toggle_key 检查（始终生效）
  │                ──► enabled 检查（禁用则放行）
  │
  └──► ActionEngine ──► ScriptRunner (shell/internal)
                      ──► KeyInjector (text_input 类型逐字符输入)

WindowFocusMonitor (平台实现) ──► ProfileManager ──► 切换 ActiveRules
                                                  ──► KeyboardHook.UpdateGrabs()
```

**数据流**：

1. `KeyboardHook` 拦截键盘事件，转换为统一 `KeyEvent`
2. `EventDispatcher` 优先检查 toggle key（始终生效），再检查 enabled 状态，然后分发到 ActionEngine
3. `WindowFocusMonitor` 监控聚焦窗口变化，通知 `ProfileManager` 切换规则集

---

## 4. 数据模型

### 4.1 KeyCombo

```go
type Modifier string // "ctrl" | "shift" | "alt" | "super" | "hyper"

type KeyCombo struct {
    Modifiers []Modifier `yaml:"modifiers"`
    Key       string     `yaml:"key"`
}
```

YAML 支持简写：`"ctrl+shift+p"` 自动解析为 `{modifiers: [ctrl, shift], key: "p"}`。

### 4.2 ActionRule

```go
type ActionType string
const (
    ActionShell    ActionType = "shell"     // 执行 shell 命令
    ActionInternal ActionType = "internal"  // 内置动作
    ActionTextInput ActionType = "text_input"  // 文本输入宏
)

type ActionRule struct {
    Trigger KeyCombo   `yaml:"trigger"`
    Type    ActionType `yaml:"type"`
    Command string     `yaml:"command"`  // shell/internal 使用
    Async   bool       `yaml:"async"`    // 是否异步执行
    Text    string     `yaml:"text"`     // text_input 类型使用
    Enter   bool       `yaml:"enter"`    // text_input: 输入后是否回车
    Delay   int        `yaml:"delay"`    // text_input: 字符间延迟(ms)
}
```

### 4.3 Profile

```go
type AppMatcher struct {
    AppID       string `yaml:"app_id"`        // 正则匹配 AppID
    ProcessName string `yaml:"process_name"`  // 正则匹配进程名
    Title       string `yaml:"title"`         // 正则匹配窗口标题
}

type Profile struct {
    Name        string          `yaml:"name"`
    Description string          `yaml:"description"`
    Match       []AppMatcher    `yaml:"match"`      // 任一匹配即激活
    IsDefault   bool            `yaml:"is_default"`
    Actions     []ActionRule    `yaml:"actions"`    // 含所有动作类型
}
```

### 4.4 Config

```go
type Config struct {
    Profiles      []Profile      `yaml:"profiles"`
    LogLevel      string         `yaml:"log_level"`
    HijackEnabled bool           `yaml:"hijack_enabled"` // 启动时是否开启劫持，默认 true
    ToggleKey     KeyCombo       `yaml:"toggle_key"`     // 运行时切换开关的按键
    Platform      PlatformConfig `yaml:"platform"`
}

type PlatformConfig struct {
    Linux   LinuxConfig   `yaml:"linux"`
    MacOS   MacOSConfig   `yaml:"macos"`
    Windows WindowsConfig `yaml:"windows"`
}

type LinuxConfig struct {
    Backend            string `yaml:"backend"`              // "auto" | "x11" | "wayland"
    CapsLockAsModifier bool   `yaml:"capslock_as_modifier"` // X11: xmodmap; Wayland: evdev 拦截
}
```

---

## 5. 配置文件

路径（所有平台统一）：`~/.config/happyboard/config.yaml`，也可通过 `-config` 指定。

### 示例

```yaml
log_level: "info"
hijack_enabled: true
toggle_key: "ctrl+shift+h"

platform:
  linux:
    backend: "auto"               # auto | x11 | wayland
    capslock_as_modifier: true

profiles:
  - name: "default"
    description: "全局默认动作"
    is_default: true
    actions:
      - trigger: "ctrl+shift+p"
        type: "shell"
        command: "rofi -show drun"
        async: true
      - trigger: "ctrl+shift+r"
        type: "internal"
        command: "reload_config"
      - trigger: "f1"
        type: "text_input"
        text: "Hello, World!"
        enter: true
      - trigger: "ctrl+f2"
        type: "text_input"
        text: |
          多行文本第一行
          多行文本第二行

  - name: "vscode"
    description: "VS Code 专用动作"
    match:
      - app_id: "Code"                    # Linux
      - app_id: "com.microsoft.VSCode"    # macOS
      - app_id: "Code.exe"                # Windows
    actions: []

  - name: "browser"
    description: "浏览器专用动作"
    match:
      - app_id: ".*(Firefox|Chrome|Chromium|Brave|Safari).*"
    actions:
      - trigger: "f2"
        type: "text_input"
        text: "https://google.com"
        enter: true

  # ─── AI 编码助手交互 ───
  # 聚焦终端中的 opencode / codex / claude code 时，一键发送常用指令
  - name: "ai-coding"
    description: "AI 编码助手快速回复"
    match:
      # 终端模拟器（按需补充你使用的终端）
      - app_id: ".*(Alacritty|kitty|foot|wezterm|gnome-terminal|konsole|iTerm|Windows Terminal).*"
      # 若 AI 工具本身有独立窗口
      - app_id: ".*(opencode|codex|claude).*"
    actions:
      # ── 快速回复 ──
      - trigger: "f1"
        type: "text_input"
        text: "OK"
        enter: true
      - trigger: "f2"
        type: "text_input"
        text: "NO"
        enter: true
      - trigger: "f3"
        type: "text_input"
        text: "Stop"
        enter: true
      - trigger: "f4"
        type: "text_input"
        text: "Continue"
        enter: true
      - trigger: "f5"
        type: "text_input"
        text: "Yes, proceed"
        enter: true
      - trigger: "f6"
        type: "text_input"
        text: "No, let me explain"
        enter: true

      # ── 常用指令 ──
      - trigger: "ctrl+f1"
        type: "text_input"
        text: "继续上面的工作"
        enter: true
      - trigger: "ctrl+f2"
        type: "text_input"
        text: "重新检查一下刚才的修改"
        enter: true
      - trigger: "ctrl+f3"
        type: "text_input"
        text: "运行测试看看结果"
        enter: true
      - trigger: "ctrl+f4"
        type: "text_input"
        text: "提交这些改动"
        enter: true
      - trigger: "ctrl+f5"
        type: "text_input"
        text: "撤销刚才的修改，换一种方案"
        enter: true

      # ── 多行模板 ──
      - trigger: "ctrl+shift+f1"
        type: "text_input"
        text: |
          请帮我审查以下代码的潜在问题：
          1. 是否有安全风险
          2. 是否有性能问题
          3. 是否有边界情况未处理
        enter: true
      - trigger: "ctrl+shift+f2"
        type: "text_input"
        text: |
          请为这个函数补充单元测试，覆盖正常路径和异常路径
        enter: true
```

> 同一 Profile 的 `match` 可包含多平台规则，运行时只匹配当前平台 AppID。

---

## 6. 平台实现

### 6.1 Linux / X11

 | 功能     | 机制                                              |
 | ------   | ------                                            |
 | 按键拦截 | `XGrabKey` - grab 动作触发组合键，阻止原始事件      |
 | 被动监听 | `XRecord` - 监听所有按键事件（辅助）              |
 | 按键注入 | `XTest` - 发送合成按键事件                        |
 | 窗口聚焦 | `_NET_ACTIVE_WINDOW` + `WM_CLASS` + `_NET_WM_PID` |

CGO 依赖：`-lX11 -lXtst -lXext`

**CapsLock 处理**（`capslock_as_modifier: true`）：启动时 `xmodmap` 重映射为 Hyper，退出时恢复。

### 6.2 Linux / Wayland

Wayland 安全模型阻止客户端全局拦截/注入按键。采用 **evdev + uinput** 内核级方案，绕过合成器限制，兼容所有 Wayland 合成器。

 | 功能     | 机制                                                 |
 | ------   | ------                                               |
 | 按键拦截 | evdev `EVIOCGRAB` - 独占键盘设备，阻止事件到达合成器 |
 | 按键注入 | uinput - 创建虚拟键盘，写入 `input_event`            |
 | 窗口聚焦 | 合成器协议（wlroots）/ DBus（GNOME/KDE）/ fallback   |

纯 Go 实现（`github.com/holoplot/go-evdev` + uinput syscalls），无需 CGO。

**拦截模型**（与 X11 的关键差异）：

```
X11:  XGrabKey 选择性拦截特定组合，其他键直达应用
Wayland: EVIOCGRAB 独占整个键盘设备，所有事件经 Hook 处理：
  - 匹配动作 -> 消费原始事件，执行动作（脚本/内置/文本输入）
  - 不匹配 -> 通过 uinput 重新注入原始事件（透传）
```

因此 `UpdateGrabs()` / `SetToggleGrab()` 在 Wayland 下为 no-op（始终 grab 全设备），过滤逻辑在事件处理循环中完成。`Pause()` / `Resume()` 通过内部标志控制：暂停时所有事件直接透传，但 toggle key 仍被检查。

**evdev 设备发现**：

```go
// 遍历 /dev/input/event*，筛选支持 EV_KEY 的设备
paths, _ := evdev.ListDevicePaths()
for _, path := range paths {
    dev, _ := evdev.Open(path)
    if dev.HasEventType(evdev.EV_KEY) {
        // 确认为键盘设备
    }
}
```

**事件处理循环**：

```go
for {
    event := dev.Read()  // 读取 evdev 事件
    keyEvent := evdevToKeyEvent(event)
    intercepted := dispatcher.HandleEvent(keyEvent)
    if !intercepted {
        uinputDev.Write(event)  // 透传：重新注入原始事件
    }
    // intercepted=true 时，动作已由 ActionEngine 执行（shell/internal/text）
}
```

**窗口聚焦检测**（按合成器适配）：

 | 合成器                     | 方案                                      | 获取信息      |
 | --------                   | ------                                    | ---------     |
 | wlroots (Sway/Hyprland 等) | `wlr-foreign-toplevel-management-v1` 协议 | app_id, title |
 | GNOME Shell                | DBus `org.gnome.Shell.Introspect`         | app_id, title |
 | KDE/KWin                   | DBus `org.kde.KWin` 脚本接口              | app_id, title |
 | 其他/未知                  | fallback: 仅默认 Profile                  | -             |

> wlroots 协议通过 wayland-client 绑定实现（CGO with libwayland-client，或纯 Go wayland 绑定）。

**CapsLock 处理**：evdev 拦截 `KEY_CAPSLOCK` (58)，标记为 modifier，不透传。无需 xmodmap。

**权限要求**：用户需在 `input` 组（读 evdev 设备）且有 `/dev/uinput` 写权限（或加入 `uinput` 组）。

### 6.3 macOS

 | 功能          | 机制                                         |
 | ------        | ------                                       |
 | 按键拦截/监听 | `CGEventTap` - 系统级事件拦截，可修改或丢弃  |
 | 按键注入      | `CGEventCreateKeyboardEvent` + `CGEventPost` |
 | 窗口聚焦      | `NSWorkspace.frontmostApplication`           |

CGO 依赖：`-framework CoreGraphics -framework ApplicationServices -framework AppKit`

**权限**：需 Accessibility 权限，启动时检测并引导。

**CapsLock**：CGEventTap 拦截 keycode 57，标记为 modifier，不影响 LED。

### 6.4 Windows

 | 功能          | 机制                                               |
 | ------        | ------                                             |
 | 按键拦截/监听 | `SetWindowsHookEx` + `WH_KEYBOARD_LL`              |
 | 按键注入      | `SendInput`                                        |
 | 窗口聚焦      | `GetForegroundWindow` + `GetWindowThreadProcessId` |

纯 Go 实现（`syscall` + `golang.org/x/sys/windows`），无需 CGO。

**CapsLock**：WH_KEYBOARD_LL 拦截 VK_CAPITAL (0x14)，阻止原始事件并标记为 modifier。

### 6.5 平台差异总结

 | 维度         | Linux/X11        | Linux/Wayland                 | macOS                           | Windows           |
 | ------       | -----------      | ---------------               | -------                         | ---------         |
 | 拦截         | XGrabKey         | evdev EVIOCGRAB               | CGEventTap                      | WH_KEYBOARD_LL    |
 | 注入         | XTest            | uinput                        | CGEventPost                     | SendInput         |
 | 拦截粒度     | 按组合键         | 整个设备                      | 全局回调                        | 全局回调          |
 | 窗口标识     | WM_CLASS         | app_id/进程名                 | Bundle ID                       | 可执行文件名      |
 | CGO          | 需要             | 不需要                        | 需要                            | 不需要            |
 | 权限         | 无特殊           | input + uinput 组             | Accessibility                   | 无                |
 | CapsLock     | xmodmap          | evdev 拦截                    | CGEventTap 拦截                 | 低级钩子拦截      |
 | Unicode 文本 | xdotool fallback | uinput 无直接支持，用按键序列 | CGEventKeyboardSetUnicodeString | SendInput UNICODE |

---

## 7. 统一按键命名 (keydef)

平台无关按键名，各平台映射到本地 keycode：

- 修饰键：`ctrl`, `shift`, `alt`, `super`(macOS=Cmd), `hyper`
- 字母/数字：`a`-`z`, `0`-`9`
- 功能键：`f1`-`f24`
- 特殊键：`space`, `enter`, `tab`, `esc`, `backspace`, `delete`, `insert`, `home`, `end`, `page_up`, `page_down`
- 方向键：`left`, `right`, `up`, `down`
- 其他：`capslock`, `print`, `scroll_lock`, `num_lock`
- 符号键：`minus`, `equal`, `left_bracket`, `right_bracket`, `backslash`, `semicolon`, `quote`, `backtick`, `comma`, `period`, `slash`

```go
type KeyMapping struct {
    Name       string
    X11Sym     int     // Linux/X11 KeySym
    EvdevCode  uint16  // Linux/Wayland evdev code
    MacCode    int     // macOS CGKeyCode
    WinVK      uint16  // Windows VK Code
}
```

各平台通过 build tags 提供 `KeyCodeFor(name)` 函数。Linux 下 X11 和 Wayland 各有独立的 keydef 文件。

---

## 8. 模块设计

### 8.1 EventDispatcher

事件处理优先级（短路求值）：

```
0. ToggleKey 检查  - 切换劫持开关（始终生效，即使劫持已禁用）
1. Enabled 检查    - 若禁用，直接放行
2. ActionEngine    - 动作匹配（shell/internal/text_input）
3. 放行            - 无匹配
```

```go
func (d *Dispatcher) HandleEvent(ev KeyEvent) bool {
    // 0. toggle key（始终检查）
    if matchCombo(ev, d.toggleKey) {
        d.toggleHijack()
        return true
    }
    // 1. 禁用则放行
    if !d.enabled {
        return false
    }
    // 2. Action 匹配
    if action := d.rules.MatchAction(ev); action != nil {
        d.actionEngine.Execute(action)
        return true
    }
    return false
}

func (d *Dispatcher) toggleHijack() {
    d.enabled = !d.enabled
    if d.enabled {
        d.hook.Resume()
    } else {
        d.hook.Pause()
    }
}
```

> `HandleEvent` 返回 `true` 时，平台 Hook 阻止原始事件传递（X11: XGrabKey 已拦截; Wayland: 不通过 uinput 透传; macOS: 回调返回 NULL; Windows: 钩子返回 1）。

### 8.2 ActionEngine

 | 类型         | 实现                   | 关键字段                 |
 | ------       | ------                 | ---------                |
 | `shell`      | 平台适配 shell 执行    | `command`, `async`       |
 | `internal`   | 内置函数               | `command`                |
 | `text_input` | KeyInjector 逐字符输入 | `text`, `enter`, `delay` |

**内置动作**：`reload_config` / `toggle_hijack` / `switch_profile:<name>`

**text_input 类型执行**：

```go
func (e *ActionEngine) runText(action *ActionRule) {
    time.Sleep(50 * time.Millisecond)
    e.injector.TypeText(action.Text, action.Delay)
    if action.Enter {
        e.injector.TapKey("enter")
    }
}
```

**Shell 平台适配**：Linux/macOS 用 `sh -c`，Windows 用 `cmd /c`。

### 8.3 ProfileManager

```go
type ActiveRules struct {
    Profile *Profile
    Actions map[string]*ActionRule
}
```

**匹配逻辑**：遍历非默认 Profile，任一 `AppMatcher` 正则匹配即激活；无匹配则用默认 Profile。

**切换流程**：匹配新 Profile → 释放旧 grab → 更新 ActiveRules → 若 enabled 则注册新 grab。

### 8.4 ConfigManager

- `Load()` / `Reload()` / `Watch()`（fsnotify 热重载）
- 默认路径：`~/.config/happyboard/config.yaml`（所有平台）
- 校验：至少一个 `is_default` Profile / KeyCombo 合法 / 无重复 trigger / toggle_key 不冲突 / text_input 类型必有 text 字段

### 8.5 劫持开关（Toggle）

```yaml
hijack_enabled: true            # 启动时是否开启劫持
toggle_key: "ctrl+shift+h"     # 运行时切换开关
```

 | 状态   | 动作 grab  | Toggle key grab   | 事件处理                 |
 | ------ | ---------- | ----------------- | ---------                |
 | 启用   | 已注册     | 已注册            | 正常匹配                 |
 | 禁用   | 已释放     | 已注册            | 除 toggle key 外全部放行 |

**平台实现**：
- Linux/X11：`Pause()` 释放动作 XGrabKey，保留 toggle key XGrabKey
- Linux/Wayland：`Pause()` 设内部标志，事件循环中跳过动作匹配，直接透传（toggle key 仍检查）
- macOS/Windows：`Pause()` 设内部标志，callback/钩子中检查后放行非 toggle 事件

**安全保证**：toggle key grab 在任何状态下保持注册，防止锁死。若未配置 toggle_key，状态固定为 `enabled` 值。

---

## 9. 项目结构

```
happyboard/
├── cmd/happyboard/main.go              # 入口
├── internal/
│   ├── config/                         # 配置结构体、解析器、校验、路径
│   ├── hook/                           # KeyboardHook 接口 + 平台实现
│   │   ├── hook.go                     # 接口定义 + KeyEvent
│   │   ├── x11/                        # //go:build linux && cgo
│   │   ├── wayland/                    # //go:build linux
│   │   ├── darwin/                     # //go:build darwin && cgo
│   │   └── windows/                    # //go:build windows
│   ├── inject/                         # KeyInjector 接口 + 平台实现
│   │   ├── injector.go
│   │   ├── x11/  wayland/  darwin/  windows/
│   ├── window/                         # WindowFocusMonitor 接口 + 平台实现
│   │   ├── monitor.go
│   │   ├── x11/  wayland/  darwin/  windows/
│   ├── dispatch/dispatcher.go          # EventDispatcher + 劫持开关
│   ├── engine/                         # ActionEngine (shell/internal/text)
│   ├── profile/                        # ProfileManager + 窗口匹配
│   ├── script/                         # Shell 命令执行
│   └── keydef/                         # 统一按键名 + 平台映射
│       ├── keydef.go
│       ├── keydef_x11.go               # //go:build linux && cgo
│       ├── keydef_wayland.go           # //go:build linux
│       ├── keydef_darwin.go            # //go:build darwin && cgo
│       └── keydef_windows.go           # //go:build windows
├── configs/example.yaml
├── go.mod  go.sum  Makefile
└── DESIGN.md
```

**平台初始化**：

```go
func newKeyboardHook(cfg LinuxConfig) hook.KeyboardHook {
    switch runtime.GOOS {
    case "linux":
        backend := cfg.Backend
        if backend == "auto" {
            backend = detectLinuxBackend() // 读取 XDG_SESSION_TYPE
        }
        switch backend {
        case "x11":      return x11hook.New()
        case "wayland":  return waylandhook.New()
        }
    case "darwin":  return darwinhook.New()
    case "windows": return winhook.New()
    }
}
// newKeyInjector() / newWindowFocusMonitor() 同理，Linux 下按 backend 选择
```

---

## 10. 依赖

**Go 依赖**：

 | 库                             | 用途                            |
 | ----                           | ------                          |
 | `gopkg.in/yaml.v3`             | YAML 配置解析                   |
 | `github.com/fsnotify/fsnotify` | 配置热重载                      |
 | `github.com/sirupsen/logrus`   | 结构化日志                      |
 | `github.com/holoplot/go-evdev` | evdev 设备读取（Linux/Wayland） |
 | `golang.org/x/sys`             | Windows API（仅 Windows）       |

**系统依赖**：
- Linux/X11：`libx11` `libxtst` `libxext`
- Linux/Wayland：无额外库（纯 Go），需 `input` + `uinput` 组权限
- macOS/Windows：系统自带

---

## 11. 关键流程

### 11.1 启动

```
main()
  ├── ConfigManager.Load() + Watch()
  ├── 创建平台实现 (Hook / Injector / Monitor)
  ├── ProfileManager.Init(config)
  ├── Dispatcher.Init(config.HijackEnabled, config.ToggleKey)
  ├── Hook.SetToggleGrab(config.ToggleKey)          # 始终注册 toggle key
  ├── WindowFocusMonitor.Start()
  │     └── 首次聚焦 → ProfileManager 匹配 → if enabled: Hook.UpdateGrabs()
  ├── Hook.Start(onEvent)
  │     └── 事件回调 → Dispatcher.HandleEvent()
  └── 信号监听 → 优雅退出
```

### 11.2 文本输入

```
用户按下 F1 (type:"text_input" action)
  → Hook 拦截 → Dispatcher → ActionEngine 匹配
  → runText: sleep 50ms → TypeText("Hello") → if enter: TapKey("enter")
```

### 11.3 Profile 切换

```
用户切换窗口 (Terminal → VS Code)
  → WindowFocusMonitor 检测变化
  → ProfileManager 匹配 "vscode" Profile
  → Hook.UpdateGrabs(): 释放旧 grab → if enabled: 注册新 grab
```

### 11.4 Toggle 切换

```
用户按下 Ctrl+Shift+H (toggle_key)
  → Dispatcher 匹配 toggle_key → toggleHijack()
  → enabled = !enabled
  → if enabled:  Hook.Resume()  (重新注册动作 grab)
  → if !enabled: Hook.Pause()   (释放动作 grab，toggle key grab 保留)
```

---

## 12. 安全与权限

 | 平台          | 权限                          | 引导                                     |
 | ------        | ------                        | ------                                   |
 | Linux/X11     | 无特殊                        | -                                        |
 | Linux/Wayland | `input` + `uinput` 组（必需） | 启动时检测，提示 `usermod` 或 `groupadd` |
 | macOS         | Accessibility 权限（必需）    | 启动时检测，引导至系统设置               |
 | Windows       | 无特殊权限                    | -                                        |

**启动参数**：
- `--safe-mode` - 不 grab 任何键，仅监听（调试用）
- `--enable` / `--disable` - 覆盖配置中的 `enabled` 值

---

## 13. 构建

```makefile
build:        ; go build -o bin/happyboard ./cmd/happyboard
build-linux:  ; CGO_ENABLED=1 GOOS=linux  go build -o bin/happyboard-linux  ./cmd/happyboard
build-macos:  ; CGO_ENABLED=1 GOOS=darwin go build -o bin/happyboard-darwin ./cmd/happyboard
build-windows:; CGO_ENABLED=0 GOOS=windows go build -o bin/happyboard.exe  ./cmd/happyboard
```

> Linux/macOS 需 CGO，须本地编译。Windows 纯 Go，可交叉编译。

---

## 14. 风险与缓解

 | 风险                               | 缓解                                  |
 | ------                             | ------                                |
 | XGrabKey 冲突                      | 日志记录 grab 失败的键                |
 | Wayland evdev 独占导致多键盘问题   | 支持同时 grab 多个键盘设备            |
 | Wayland 窗口聚焦不可用             | fallback 至默认 Profile，日志提示     |
 | macOS 权限未授予 / EventTap 被禁用 | 启动检测 + 引导授权 + 监听状态        |
 | Windows 钩子超时(>300ms)           | 回调内只做快速匹配，耗时操作异步      |
 | 配置错误锁死键盘                   | `--safe-mode` + toggle key 始终可切换 |
 | toggle_key 未配置且 enabled=false  | 启动警告 + `--enable` 参数覆盖        |
 | CapsLock 重映射影响系统            | 退出时恢复原始状态                    |
 | 平台按键映射差异                   | keydef 统一映射表 + 逐平台测试        |

---

## 15. 未来扩展

- GUI 配置工具
- 按键序列映射（Vim 风格 `g g` -> `Home`）
- 按住超时映射
- 按键使用统计
