# happyboard

跨平台热键控制器——按键触发 shell 命令 / 内置动作 / 文本输入，支持应用感知 Profile 自动切换。

## 功能

 | 类型         | 说明                                                                  |
 | ---          | ---                                                                   |
 | `shell`      | 按键触发 shell 命令（支持异步）                                       |
 | `internal`   | 内置动作：`reload_config` / `toggle_hijack` / `switch_profile:<name>` |
 | `text_input` | 按键输入文本宏（支持多行、延迟、回车）                                |
 | Profile      | 根据聚焦窗口自动匹配动作方案                                          |
 | Toggle       | 运行时开关劫持（热键切换 + 默认状态配置）                             |

## 快速开始

```yaml
# ~/.config/happyboard/config.yaml
log_level: "info"
hijack_enabled: true
toggle_key: "ctrl+shift+x"

profiles:
  - name: "default"
    is_default: true
    actions:
      - trigger: "ctrl+shift+p"
        type: "shell"
        command: "rofi -show drun"
        async: true
      - trigger: "f1"
        type: "text_input"
        text: "Hello, World!"
        enter: true
```

```bash
happyboard                    # 启动
happyboard -config path.yaml  # 指定配置
```

> 启动时劫持开关状态由配置文件中 `hijack_enabled` 字段控制。

## 平台

 | 平台          | 后端                       | CGO    |
 | ---           | ---                        | ---    |
 | Linux/X11     | XGrabKey + XTest           | 需要   |
 | Linux/Wayland | evdev + uinput             | 不需要 |
 | macOS         | CGEventTap                 | 需要   |
 | Windows       | WH_KEYBOARD_LL + SendInput | 不需要 |

## 构建

```bash
make                # go build
make build-cgo      # CGO_ENABLED=1
make build-nocgo    # CGO_ENABLED=0
make run            # 构建并启动 (configs/example.yaml)
```

## 配置参考

见 [`configs/example.yaml`](configs/example.yaml)。

完整设计文档见 [`DESIGN.md`](DESIGN.md)。
