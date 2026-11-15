<div align="center">

<img src="assets/neru-appicon.png" alt="Neru" width="80" />

# Neru

为你的整个桌面提供提示标签、网格和 vim 按键。免费、开源、单个二进制文件、单个 TOML 配置文件。

[![Latest Release](https://img.shields.io/github/v/release/y3owk1n/neru?style=flat-square)](https://github.com/y3owk1n/neru/releases)
[![License](https://img.shields.io/github/license/y3owk1n/neru?style=flat-square)](LICENSE)
[![Discord](https://img.shields.io/discord/1502261043701874698?style=flat-square&label=discord)](https://discord.gg/KZwnwr9dz6)
[![Sponsor](https://img.shields.io/badge/sponsor-%E2%9D%A4-30363D?style=flat-square)](https://github.com/sponsors/y3owk1n)

|     macOS 14+     |   Linux (X11, Wayland)   |     Windows 10+     |
| :---------------: | :----------------------: | :-----------------: |
|     稳定 ✅       |    测试版，每日更新 🔵  |  测试版，每日更新 🔵 |

<sub>[这些标签的含义](docs/reference/platform-support.md#what-the-labels-mean)</sub>

</div>

---

https://github.com/user-attachments/assets/322e2666-d961-46a9-bd7c-39e42fda8819

Neru 做的就是 Vimium 在浏览器里做的事，但作用于屏幕上的每个应用、窗口、菜单栏和程序坞项目。按一下热键，输入你想点击的元素上的标签，光标就会跳过去并点击。

## 为什么选择 Neru

- **在无障碍树失效的地方也能用。** 网格模式切分的是像素而非控件，所以在画布、游戏和远程桌面中也能工作。提示（Hints）使用无障碍树、设备端 OCR，或纯 Go 实现的屏幕图像扫描。
- **快。** 事件监听覆盖每一次按键，Neru 把任何新增延迟都视为 bug。
- **不做逐应用的特殊处理。** 提供良好的默认值，加上你自己编写的逐应用覆盖，在焦点切换时生效。
- **可脚本化。** 热键、`neru` CLI 和 IPC 套接字执行同一套命令，所以一个绑定在 skhd、Hammerspoon、Raycast 或 shell 脚本中都能原样工作。当模式打开或关闭、焦点应用改变或 Neru 暂停时，钩子会运行你自己的步骤。`neru watch` 实时流式输出同样的事件，可用 `--event` 过滤。`neru query` 报告显示器、光标和提示目标；状态和查询命令带 `--json` 时会给程序提供可解析的输出。IPC 协议带版本号，附 JSON schema 和 Go 客户端包。
- **注重隐私。** OCR 和元素检测在设备端运行。无遥测、无账户，键入的文字永远不会进入日志。
- **大声失败。** 绑定中的拼写错误标志会在配置校验时报错，而不是按下按键后毫无反应。

## 安装

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.sh | bash
```

```powershell
# Windows
irm https://raw.githubusercontent.com/y3owk1n/neru/main/scripts/install.ps1 | iex
```

再次运行即可更新。Homebrew、oku、Nix、预编译二进制、从源码构建、[代理技能](docs/guide/installation.md#set-up-with-an-agent)和卸载方法见[安装](docs/guide/installation.md)。

## 快速开始

运行 `neru launch`，授予 `neru doctor` 要求的权限，然后按
`Primary+Shift+Space` 呼出提示。包括 Linux 在内的完整首次运行流程见
[快速开始](docs/guide/getting-started.md)。

## 模式

<table>
<tr>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/4327c027-d3c8-4915-a740-a0c9c46b386d" alt="Hints Mode" /><br/>
<sub><b>提示（Hints）</b></sub>
</td>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/3b8a4ccb-f102-4971-902e-c4097d14afe5" alt="Vim Mode" /><br/>
<sub><b>Vim</b></sub>
</td>
</tr>
<tr>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/f72b43fb-9745-4c17-a80d-15ca4077a5a2" alt="Recursive Grid Mode" /><br/>
<sub><b>递归网格（Recursive Grid）</b></sub>
</td>
<td align="center" width="50%">
<img src="https://github.com/user-attachments/assets/6b18396a-9bb8-4d1b-a8f8-270c19c0f727" alt="Bisect Mode" /><br/>
<sub><b>二分（Bisect）</b></sub>
</td>
</tr>
<tr>
<td align="center" colspan="2">
<img src="https://github.com/user-attachments/assets/b0759650-a3fa-43f3-9123-237124afe870" alt="Grid Mode" width="50%" /><br/>
<sub><b>网格（Grid）</b></sub>
</td>
</tr>
</table>

| 模式                  | 默认热键             | 工作方式                                                          | 最适合                                    |
| :-------------------- | :------------------- | :---------------------------------------------------------------- | :---------------------------------------- |
| **递归网格** ⭐        | `Primary+Shift+C`     | 3x3 切分，一格接一格地选择，直到光标落点                          | 任何场景，包括画布和游戏                  |
| **二分**               | `Primary+Shift+B`     | `hjkl` 保留一半，`yubn` 保留一个象限，直到光标落点                 | 任何场景，无需阅读标签                    |
| **提示**               | `Primary+Shift+Space` | 通过无障碍树、OCR 或像素扫描为每个目标打标签                      | 原生应用、Electron、浏览器、Figma         |
| **网格**               | `Primary+Shift+G`     | 输入行列标签跳到对应单元格                                        | 在大显示器上粗粒度跳转                    |
| **滚动**               | `Primary+Shift+S`     | 在任意滚动视图里使用 `j`/`k`、`u`/`d`、`gg`/`G`、`h`/`l`          | 双手不离键盘地阅读                        |
| **Vim**                | 未绑定               | 读取屏幕文字并用 vim 按键在其上移动光标                            | 选择、复制和编辑任意应用中的文字          |
| **显示器选择**          | 已禁用、未绑定       | 为每个显示器打标签，输入一个即可跳转                              | 多显示器环境                              |
| **自定义**              | 未绑定               | 你在配置中声明的名称、指示器和按键表                              | 窗口管理分层、应用专属按键                |

`Primary` 在 macOS 上是 `Cmd`，在 Linux 和 Windows 上是 `Ctrl`。Linux 版本不带这些默认值，避免与终端快捷键冲突。所有绑定都可重新映射。

除显示器选择外的每个模式都提供左键、右键和中键点击，双击和三击，任意按键拖拽，以及粘滞修饰键。`[held_repeat]` 提供按住滑行，需要的话还带加速。

## 配置

一切都在一个 TOML 文件里。用 `neru config reload` 应用修改，或用 `neru config set` 修改单个值。

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"   # 选中即点击，Vimium 风格
"Primary+Shift+W" = "mode window"

[macros]
click_and_exit = ["action left_click --bail-on-error", "idle"]

[[hints.app_configs]]
bundle_id = "com.brave.Browser"   # macOS 上是 bundle ID，Linux 上是 WM_CLASS 或 app_id，Windows 上是可执行文件完整路径
hotkeys = { "Return" = "macro click_and_exit" }

[modes.window]
indicator = "Window"

[modes.window.hotkeys]
"h" = "exec yabai -m window --focus west"
"l" = "exec yabai -m window --focus east"
```

每条热键命令也是 shell 命令：

```bash
neru hints --role button --text save   # 只匹配文字为 save 的按钮
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit" "action restore_cursor_pos"
neru config set hints.strategy contour
neru doctor                            # 诊断权限和后端
```

主题、指示器、平滑光标与滚动、虚拟指针、应用排除和屏幕共享隐藏也都是可选项。

## 基于 Neru 构建

状态栏、启动器和语音工具都可以跟随并驱动 Neru：

```bash
neru watch --event mode_enter,mode_exit   # 每次模式切换输出一行 JSON
```

```go
neru := client.New()   // github.com/y3owk1n/neru/client
status, err := neru.Status(ctx)
_, err = neru.Send(ctx, "hints", "--action", "left_click")
```

哪种接口适合哪种工具见[基于 Neru 构建](docs/guide/integrations.md)。配合编程代理使用时，`neru-build-integration` [技能](docs/guide/installation.md#set-up-with-an-agent)会针对你安装的版本编写并测试一个集成。

## 文档

在 **[neru.kylewong.my](https://neru.kylewong.my/)** 阅读文档，
或在 GitHub 上浏览 [docs/](docs/README.md)。

## 社区

> _"neru：摆脱鼠标的真正最佳应用"_

[![Community YouTube Review](https://img.youtube.com/vi/OFnpYTDA2gY/maxresdefault.jpg)](https://www.youtube.com/watch?v=OFnpYTDA2gY)

- [Neru Dojo](https://bernatgene.github.io/neru-dojo/)：训练递归网格和提示的浏览器小游戏。
- [社区配置](docs/project/showcases.md)：用户分享的设置。
- [Discord](https://discord.gg/KZwnwr9dz6)：提问与配置交流。

## 贡献

欢迎提交 bug 报告（尤其是 Linux 和 Windows 上的）和 pull request。从 [CONTRIBUTING.md](CONTRIBUTING.md) 开始。

## 支持项目

如果 Neru 已经进入你的工作流，请[赞助它](https://github.com/sponsors/y3owk1n)。只需要窗口管理？[mimi](https://github.com/y3owk1n/mimi) 是把 Neru 的窗口和空间功能独立出来的工具。

## 许可证

MIT。见 [LICENSE](LICENSE)。

<div align="center">
<br/>

由 <a href="https://github.com/y3owk1n">y3owk1n</a> 用 ❤️ 制作

</div>
