# SGFG-VUE

A private proxy client for Windows, rewritten in Go. [GoST v3](https://github.com/go-gost/gost) and [Hysteria2](https://github.com/apernet/hysteria) are compiled directly into a single-file executable as Go libraries — no external core processes. The UI is built with Wails v3 + Vue 3 + TypeScript in a light-green candy theme with rich animations.

**Current version: v11.2**

> ## ⚠️ Legal Notice / Node Information
>
> **To comply with the laws and regulations of the People's Republic of China, this open-source repository does NOT include any server node information.** All node hostnames, passwords, and obfuscation secrets have been removed from the published source code.
>
> This software is a **client only**. To use it, you must deploy and operate your own proxy servers and fill in your own node configuration in `internal/core/nodes.go` before building (see [Node Configuration](#node-configuration)). The authors do not provide, sell, or endorse any proxy service.

## Features

| Feature | Description |
|---|---|
| WebTransport transport | GOST `relay+wt` (WebTransport/HTTP3) direct connection |
| Hysteria2 transport | hy2 direct connection with salamander obfuscation and port hopping |
| Turtleor transport | hy2 via warpplus (M-H3) with salamander obfuscation and port hopping |
| Node selection | Random selection per transport across configured nodes, with ICMP latency probing and automatic failover |
| Local proxy | HTTP or SOCKS5 (mutually exclusive), listening on 127.0.0.1 or 0.0.0.0 (LAN access), default port 7788 with automatic increment if occupied |
| System proxy | Automatically sets/restores the Windows system proxy on connect/disconnect; detects and repairs stale settings after abnormal exits |
| Options window | Separate Wails window for proxy options, custom port, and the SS relay |
| SS LAN relay | After connecting, exposes a `ss://aes-128-gcm` entrance forwarding to the local proxy, so other LAN devices can share the connection |
| Password generator | Random relay passwords (8-16 alphanumeric characters) |

## Tech Stack

- **Backend**: Go 1.25+
  - `github.com/go-gost/x` — GOST v3 core library
  - `github.com/apernet/hysteria/core/v2` + `extras/v2` — Hysteria2 client + salamander obfuscation
  - `github.com/wailsapp/wails/v3` — desktop application framework
  - `golang.org/x/sys` — Windows registry access
- **Frontend**: Vue 3 + TypeScript + Vite, Bun as package manager, `@wailsio/runtime`

> Note: dependency versions in `go.mod` are **pinned deliberately** to stay protocol-compatible with a gost v3.1.0-era server stack (older relay/WebTransport behavior). Do not upgrade `go-gost/*`, `quic-go/*`, or `apernet/hysteria` unless your servers are upgraded accordingly.

## Node Configuration

All node and server parameters live in `internal/core/nodes.go`. Before building, edit it to match your own servers:

```go
var Nodes = []Node{
	{Name: "node1", Host: "node1.example.com", Password: "your-password"},
	{Name: "node2", Host: "node2.example.com", Password: "your-password"},
	// add as many as you like; each transport picks one at random
}

const (
	GostServerPort = 1443                  // your gost relay+wt server port
	ObfsHysteria2  = "your-hy2-obfs-pass"  // salamander obfs password for hysteria2
	ObfsTurtleor   = "your-warp-obfs-pass" // salamander obfs password for turtleor
)
```

Other tunables in the same file:

- `LocalProxyPort` (default 7788) and `LocalProxyPortMaxTry` — local listen port and auto-increment range
- Port hopping ranges in `BuildSelection()`: hysteria2 uses a random port in 30000-50000, turtleor in 10000-20000 — align these with your server-side port hopping configuration

Server-side requirements:

- **WebTransport**: a gost v3.1.0-compatible `relay+wt` listener on `GostServerPort`, with user `dwg` and the per-node `Password` (client URL: `relay+wt://dwg:<password>@<host>:<port>?secure=true`)
- **Hysteria2 / Turtleor**: hysteria2 server with salamander obfuscation enabled, password = node `Password`, and port hopping covering the ranges above

## Build Requirements

| Requirement | Version |
|---|---|
| OS | Windows 10/11 (sysproxy and ICMP ping are Windows-only implementations) |
| Go | 1.25+ (per `go.mod`) |
| Bun | 1.x, available in `PATH` |
| Wails CLI | v3 (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`) |
| WebView2 Runtime | required at runtime (preinstalled on most Win10/11 systems) |

## Build Commands

```powershell
# install frontend dependencies (first time only)
cd frontend; bun install; cd ..

# production build: generates bindings, builds the frontend with bun,
# generates icons/syso, and compiles the exe
wails3 build
# output: bin/sgfg-ng.exe (single file, ~28 MB)

# development mode (frontend hot reload + Go recompile)
wails3 dev

# regenerate frontend bindings after changing exported Go methods
wails3 generate bindings -ts

# checks
go vet ./...
go test ./...
cd frontend; bun run typecheck
```

## Server Deployment

Each node is a Linux server (Debian/Ubuntu shown below) running two services: **GoST** (WebTransport transport) and **Hysteria2** (Hysteria2 + Turtleor transports). You need a domain name per node pointing at the server and a valid TLS certificate.

### 0. TLS certificate

Both GoST (`secure=true` on the client) and Hysteria2 require a trusted certificate:

```bash
sudo apt install certbot
sudo certbot certonly --standalone -d node1.example.com
# cert: /etc/letsencrypt/live/node1.example.com/fullchain.pem
# key:  /etc/letsencrypt/live/node1.example.com/privkey.pem
```

### 1. GoST (WebTransport transport)

Download **gost v3.1.0** (this exact era is required — the client's pinned libraries are protocol-compatible with it, not with newer releases):

```bash
curl -L -o gost.tar.gz https://github.com/go-gost/gost/releases/download/v3.1.0/gost_3.1.0_linux_amd64.tar.gz
tar xzf gost.tar.gz && sudo mv gost /usr/local/bin/gost && sudo chmod +x /usr/local/bin/gost
```

Run a `relay+wt` listener on UDP port 1443. The client connects as user `dwg` with the per-node password from `nodes.go`:

```bash
gost -L "relay+wt://dwg:NODE_PASSWORD@:1443?cert=/etc/letsencrypt/live/node1.example.com/fullchain.pem&key=/etc/letsencrypt/live/node1.example.com/privkey.pem"
```

systemd unit `/etc/systemd/system/gost.service`:

```ini
[Unit]
Description=gost relay+wt
After=network.target

[Service]
ExecStart=/usr/local/bin/gost -L "relay+wt://dwg:NODE_PASSWORD@:1443?cert=/etc/letsencrypt/live/node1.example.com/fullchain.pem&key=/etc/letsencrypt/live/node1.example.com/privkey.pem"
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now gost
```

### 2. Hysteria2 (Hysteria2 transport)

Install via the official script (creates the `hysteria-server` systemd service):

```bash
bash <(curl -fsSL https://get.hy2.sh/)
```

Edit `/etc/hysteria/config.yaml` — salamander obfs password must equal `ObfsHysteria2` in `nodes.go`, auth password must equal the node's `Password`:

```yaml
listen: :443

tls:
  cert: /etc/letsencrypt/live/node1.example.com/fullchain.pem
  key: /etc/letsencrypt/live/node1.example.com/privkey.pem

auth:
  type: password
  password: NODE_PASSWORD

obfs:
  type: salamander
  salamander:
    password: HY2_OBFS_PASSWORD

masquerade:
  type: proxy
  proxy:
    url: https://www.bing.com
    rewriteHost: true
```

```bash
sudo systemctl enable --now hysteria-server
```

**Port hopping** — the client dials a random UDP port in 30000-50000; redirect that range to the real listen port:

```bash
sudo iptables -t nat -A PREROUTING -i eth0 -p udp --dport 30000:50000 -j DNAT --to-destination :443
sudo apt install iptables-persistent   # to survive reboots
```

### 3. Turtleor (Hysteria2 through Cloudflare WARP)

The Turtleor transport is a second Hysteria2 instance whose outbound traffic exits through Cloudflare WARP instead of the server IP directly.

**Install and start warp-cli:**

```bash
curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | sudo gpg --yes --dearmor -o /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/cloudflare-client.list
sudo apt update && sudo apt install cloudflare-warp

warp-cli registration new
warp-cli mode proxy          # expose a local SOCKS5 endpoint instead of tunneling the whole host
warp-cli proxy port 40000
warp-cli connect
warp-cli account             # verify: Account type: Free
```

**Second Hysteria2 instance** — create `/etc/hysteria/warp.yaml` (note `ObfsTurtleor` is different from `ObfsHysteria2`), routing all traffic through the WARP SOCKS5 endpoint via `outbounds` + `acl`:

```yaml
listen: :8443

tls:
  cert: /etc/letsencrypt/live/node1.example.com/fullchain.pem
  key: /etc/letsencrypt/live/node1.example.com/privkey.pem

auth:
  type: password
  password: NODE_PASSWORD

obfs:
  type: salamander
  salamander:
    password: TURTLEOR_OBFS_PASSWORD

outbounds:
  - name: warp
    type: socks5
    socks5:
      addr: 127.0.0.1:40000

acl:
  inline:
    - warp(all)

masquerade:
  type: proxy
  proxy:
    url: https://www.bing.com
    rewriteHost: true
```

```bash
sudo systemctl enable --now hysteria-server@warp   # instance name = config file name
sudo iptables -t nat -A PREROUTING -i eth0 -p udp --dport 10000:20000 -j DNAT --to-destination :8443
```

### 4. Firewall

Open (also in your cloud provider's security group):

| Port | Protocol | Used by |
|---|---|---|
| 1443 | UDP | GoST relay+wt (QUIC) |
| 443 | UDP | Hysteria2 direct |
| 30000-50000 | UDP | Hysteria2 port hopping |
| 8443 | UDP | Turtleor instance |
| 10000-20000 | UDP | Turtleor port hopping |
| 80/443 | TCP | certbot / masquerade (temporary) |

## Client Build & Deployment

1. Edit `internal/core/nodes.go` with your own servers (see [Node Configuration](#node-configuration)); passwords and obfs secrets must match the server configs above.
2. Run `wails3 build`.
3. Distribute `bin/sgfg-ng.exe` — it is a single self-contained file; no installation or external cores are needed.
4. On first run, pick a transport, connect, and optionally open the options window (gear icon) for SOCKS5 mode, LAN access, custom port, or the SS relay.

## Project Structure

```
sgfg-ng/
├── main.go                     # Wails entry: windows, service registration, embedded assets
├── service.go                  # ProxyService: methods bound to the frontend
├── build/                      # wails3 build assets (icon, syso, product info)
├── internal/
│   ├── core/
│   │   ├── nodes.go            # node table, transports, port hopping, password generator
│   │   ├── gost.go             # GOST library wrapper: CLI-style URLs -> service instances
│   │   ├── hysteria.go         # Hysteria2 client wrapper (lazy reconnect + salamander)
│   │   └── localproxy.go       # local SOCKS5 (incl. UDP ASSOCIATE) / HTTP CONNECT server
│   ├── proxy/manager.go        # core state machine: select/connect/disconnect/relay/sysproxy
│   ├── sysproxy/               # Windows system proxy (registry + wininet refresh)
│   └── ping/                   # ICMP ping via iphlpapi IcmpSendEcho (no admin needed)
└── frontend/
    ├── bindings/               # generated by wails3 (do not edit)
    └── src/
        ├── views/              # MainView / OptionsView
        ├── components/         # candy UI controls + turtle/scan/connect animations
        └── bridge.ts           # backend calls / event subscriptions
```

## Known Limitations

- Windows only (system proxy and ping are Windows-specific).
- The Hysteria2 local SOCKS5 UDP ASSOCIATE implementation is simplified: fragmented (FRAG != 0) packets are dropped per RFC intent; the UDP relay socket binds to the local address of its TCP control connection (works for LAN clients when LAN access is enabled).
- Local connections are capped at 512 concurrent sessions; HTTP CONNECT headers are capped at 64 KB.
- Node information is hardcoded by design; changing nodes requires editing `internal/core/nodes.go` and rebuilding.

## Acknowledgments

This project was built in collaboration with AI pair-programming and review models:

- **Kimi K3** — co-developer: full Go rewrite, backend architecture, UI implementation, debugging, and documentation
- **GPT 5.6 Luna** — code review
- **Grok 4.5** — code review

## License

This project's own code is released under the **MIT-0 (MIT No Attribution)** license — see [LICENSE](LICENSE). You may use it freely, with no attribution required.

**Important:** the bundled dependencies [GoST](https://github.com/go-gost/gost) and [Hysteria2](https://github.com/apernet/hysteria) are licensed under the **MIT license**, which *does* require retaining their copyright and license notices. When redistributing this software (source or binary), you must include the [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES) file shipped in this repository.
