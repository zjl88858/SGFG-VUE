# SGFG TUI

`SGFG TUI` 是 `./gui-code` 中 SGFG-NG v11.2 的跨平台终端移植版。界面使用 Go 与 Charm 组件栈（Bubble Tea、Bubbles、Lip Gloss），GoST v3 和 Hysteria2 仍作为 Go 库编译进单个可执行文件，不启动外部代理核心。

支持 Windows、Linux 和 macOS。原 Wails/Vue 工程保留在 `./gui-code`，仅作为行为与设计参考。

## 功能对应

| 功能 | TUI 实现 |
|---|---|
| WebTransport | GOST `relay+wt` 直连，远端端口 1443 |
| Hysteria2 | hy2 直连，Salamander 混淆，端口跳跃 30000-50000 |
| Turtleor | hy2 经 warpplus，端口跳跃 10000-20000 |
| 节点选择 | 随机选择节点，每节点最多探测 3 次，失败自动换节点 |
| 本地代理 | HTTP 或 SOCKS5，回环或 LAN 监听，默认 7788，占用后尝试后续 9 个端口 |
| 系统代理 | 按平台自动配置，连接前保存原状态，正常断开或退出时恢复 |
| SS 中继 | `ss://aes-128-gcm` 入口，默认端口 8388，可生成随机密码 |
| 状态与日志 | 节点、跳跃端口、延迟、本地监听状态和最近 60 条日志 |

ICMP 探测失败不会阻止连接。这一点与 GUI 版一致，因为部分网络会屏蔽 ICMP，但代理传输仍可能正常工作。

## 平台行为

| 平台 | ICMP | 自动系统代理 |
|---|---|---|
| Windows | `IcmpSendEcho`，无需管理员权限 | WinINET 当前用户注册表，并刷新设置 |
| macOS | 非特权 ICMP datagram socket | `networksetup`，同时设置 HTTP/HTTPS 代理 |
| Linux | 非特权 ICMP datagram socket | GNOME/Unity/Cinnamon/Pantheon/Budgie 下使用 `gsettings` |

Linux 没有统一的桌面系统代理接口。非 GNOME 系桌面会在 TUI 中明确显示自动系统代理不可用，此时本地 HTTP/SOCKS5 代理、三种传输和 SS 中继仍全部可用，需要在应用或桌面环境中手动填写代理地址。

macOS 的 `networksetup` 可能受设备管理策略或用户权限限制；失败时连接会回滚，不会留下已启动但不可见的本地代理。

系统代理快照目前保存在进程内，可覆盖正常断开、`q`、Ctrl+C、SIGTERM 和多数正常退出路径。操作系统强制终止或断电无法执行恢复；与 GUI 版按固定端口盲目清理相比，本版不会误关用户原有的本地代理。遇到强制退出时，请在系统设置中手动关闭或恢复代理。

## 快捷键

| 快捷键 | 操作 |
|---|---|
| `1` / `2` / `3` | 选择 WebTransport / Hysteria2 / Turtleor |
| `c` 或 `Enter` | 建立或断开连接 |
| `Tab`、`Shift+Tab`、`←`、`→` | 切换主页、选项和 SS 中继页面 |
| `h` / `o` / `r` | 直接打开主页 / 选项 / 中继页 |
| `↑` / `↓`、`j` / `k` | 在选项页选择项目 |
| `Space` 或 `Enter` | 切换选项或编辑输入框 |
| `g` | 生成 SS 中继密码 |
| `Esc` | 结束输入 |
| `q` 或 Ctrl+C | 清理代理资源并退出 |

鼠标操作已启用：可以点击顶部页面、传输方式、连接操作条、选项和中继控件；在主页 Activity 区域使用滚轮可浏览最近 60 条日志。

警告和错误会显示为 TUI 内居中的模态确认框，不创建操作系统窗口。按 `Enter`、`Esc`、`q`，或点击确认按钮即可关闭。

## 构建

需要 Go 1.25+。依赖包含纯 Go 网络栈，不要求 CGO、Wails、Bun、Node.js、WebView 或外部 GoST/Hysteria 可执行文件。

```bash
go test ./...
go build -trimpath -o sgfg-tui .
```

交叉编译示例：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o sgfg-tui.exe .
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -o sgfg-tui-linux .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -o sgfg-tui-macos .
```

运行：

```bash
./sgfg-tui
```

默认情况下 GoST 底层日志保持静默，与 Hysteria2 一样仅在 TUI 中显示连接摘要，避免直接写入 stderr 破坏界面。设置 `SGFG_GOST_DEBUG=1` 可临时启用 GoST debug 日志用于排障；该模式会直接输出详细底层日志，不适合日常 TUI 使用。节点、密码、混淆密码和端口范围仍位于 `internal/core/nodes.go`，保持与 GUI 版一致。

## 依赖锁定与历史坑

服务器使用 gost v3.1.0 时代的协议栈。以下版本是协议兼容要求，不是普通的保守锁定：

| 模块 | 版本 | 原因 |
|---|---:|---|
| `go-gost/x` | v0.5.5 | 与 gost v3.1.0 时代实现一致 |
| `go-gost/core` | v0.3.1 | 与 `x v0.5.5` 配套 |
| `go-gost/relay` | v0.5.0 | v0.6.x 会被旧服务端以 status 500 拒绝 |
| `quic-go/quic-go` | v0.53.0 | 新版要求旧服务端未启用的 partial delivery |
| `quic-go/webtransport-go` | v0.9.0 | 与 quic-go v0.53 配套 |
| `quic-go/qpack` | v0.5.1 | 避免与旧 QUIC 栈冲突 |
| Hysteria core/extras | v2.6.5 | 保持与上述 qpack/QUIC 组合兼容 |

不要单独升级这些依赖，除非服务端协议栈同步升级并重新完成三种传输的互通测试。历史上错误升级曾分别导致：

```text
webtransport: stream reset partial delivery required
received status 500
```

`internal/core/gost.go` 中的 blank imports、每实例唯一 service/chain/hop 名称及“不使用 `loader.Load`”同样必须保留。遗漏协议注册会发生静默 scheme 回退；使用全局 loader 会在启动 SS 中继时关闭主代理实例。

Hysteria 的应用层本地代理处于其 `internal` 包中，因此本项目继续维护简化的 HTTP CONNECT 与 SOCKS5（含 UDP ASSOCIATE）入口。UDP `FRAG != 0` 的分片会被丢弃，并发连接上限为 512，HTTP CONNECT 请求头上限为 64 KiB，均与 GUI 版一致。

## 与 GUI 版的有意差异

- 移除 Wails、Vue、WebView2、多窗口和窗口动画，改为三个 TUI 页面。
- 节点扫描、连接、断开、周期 Ping 和中继操作均由 Bubble Tea 异步命令执行，不阻塞键盘和重绘。
- 系统代理由平台能力决定默认值；不支持自动配置的 Linux 桌面默认关闭。
- SOCKS5/系统代理互斥、连接期间禁止修改选项现在由后端状态机强制执行，不再只依赖 UI。
- Windows 断开时恢复原 `ProxyEnable`、`ProxyServer` 和 `ProxyOverride`，而非简单设置 `ProxyEnable=0`。

## 项目结构

```text
.
├── main.go                    # TUI 入口、信号处理和统一 Shutdown
├── internal/
│   ├── core/                  # GOST、Hysteria2、本地 HTTP/SOCKS5、节点数据
│   ├── ping/                  # Windows 与 Unix ICMP 实现
│   ├── proxy/                 # 平台无关代理状态机
│   ├── sysproxy/              # Windows/macOS/Linux 系统代理适配
│   └── tui/                   # Bubble Tea Model/Update/View
└── gui-code/                  # 原 Wails/Vue 参考实现
```
