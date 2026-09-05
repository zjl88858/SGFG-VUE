# SGFG-VUE

A proxy client for Windows, written in Go. [GoST v3](https://github.com/go-gost/gost) and [Hysteria2](https://github.com/apernet/hysteria) are compiled into the application as Go libraries — no external proxy core processes. Optional process proxying uses a separate NetFilter SDK runtime and kernel driver. The UI is built with Wails v3 + Vue 3 + TypeScript in a light-green candy theme.

**Current version: v11.3**

> ## Server Configuration and Licensing
>
> Server addresses, credentials and obfuscation settings are maintained in `internal/core/nodes.go`. Review this file before publishing your own repository or binaries. Documentation examples below use placeholders; they are not a proxy service.
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
| Process proxy | Windows x64: proxy selected `.exe` names; other processes bypass. Case-insensitive exact filenames, including future instances. Mutually exclusive with the system proxy |
| Driver lifecycle | When process proxy is enabled, connecting installs and starts the application-owned demand-start driver; disconnecting detaches filtering, unloads the driver and removes its service/file. Normal proxy connections do not install a driver |
| Process DNS | Captured UDP/TCP port 53 requests use `tcp://1.1.1.1:53` through the selected node; UDP requests are translated to TCP and replies restored to the original DNS peer |
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
- **Optional process filtering**: NetFilter SDK SockFilter C API (Windows x64). This is a **commercial, separately licensed dependency**, not an open-source component of SGFG. Obtain appropriate runtime/driver distribution rights from the vendor.

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

- **WebTransport**: a compatible `relay+wt` listener on `GostServerPort`, with user `dwg` and the per-node `Password` (client URL: `relay+wt://dwg:<password>@<host>:<port>?secure=true`). The configured custom server supports TCP only. The client therefore disables SOCKS5 UDP ASSOCIATE for this transport; process DNS uses TCP, and other non-local UDP traffic is dropped instead of leaking directly. QUIC transport packets to the node are still UDP; this restriction concerns proxied application traffic.
- **Hysteria2 / Turtleor**: hysteria2 server with salamander obfuscation enabled, password = node `Password`, and port hopping covering the ranges above

## Build Requirements

| Requirement | Version |
|---|---|
| OS | Windows 10/11 (sysproxy and ICMP ping are Windows-only implementations) |
| Go | 1.25+ (per `go.mod`) |
| Bun | 1.x, available in `PATH` |
| Wails CLI | `v3.0.0-alpha2.117`, aligned with `go.mod` |
| WebView2 Runtime | required at runtime (preinstalled on most Win10/11 systems) |
| Administrator privileges | The Windows 11.3 executable requests elevation at startup to manage its kernel driver; UAC/security policies are not bypassed |
| NetFilter SDK (optional) | Compatible x64 SockFilter C API DLL and a driver whose signature is accepted by the target Windows system |

## Build Commands

```powershell
# install frontend dependencies (first time only)
cd frontend; bun install; cd ..

# production build: generates bindings, builds the frontend with bun,
# generates icons/syso, and compiles the exe
wails3 build
# output: bin/sgfg-ng.exe plus bin/sockfilter/ when SDK assets are available

# development mode (frontend hot reload + Go recompile)
wails3 dev

# regenerate frontend bindings after changing exported Go methods
wails3 generate bindings -ts

# checks
go vet ./...
go test ./...
cd frontend; bun run typecheck
bun test
```

### Optional SDK assets

Place the vendor SDK in `nfsdk/` (or supply the corresponding runtime files yourself). The native Windows build stages:

```text
bin/
  sgfg-ng.exe
  sockfilter/
    nfapi.dll                    # sockfilter/bin/release_c_api/x64/nfapi.dll
    driver/x64/sockfilter.sys     # sockfilter/bin/driver/x64/sockfilter.sys
    license.rtf
```

`build/windows/stage-sockfilter.ps1` copies assets only; it never installs a driver. Without SDK files, the ordinary proxy build remains available and process proxying reports the missing dependency when enabled. Do not commit or redistribute vendor binaries/source unless your license permits it. The bundled SDK used for internal evaluation has connection limits and test-signing restrictions; preparing a production UI does not turn an evaluation driver into a production-licensed driver.

Use the **x64 C API** `nfapi.dll`, not the SDK's C++ or Win32 DLL. The application loads it only from `sockfilter/nfapi.dll` beside the executable, not from `PATH` or the working directory; the integration does not require CGO or an external native helper. The separate internal `README.sockfilter.md` is optional and need not be published: this README contains the essential deployment and safety information. The staging script copies that extra document only when present.

### Using process proxy

1. Launch the application and approve the Windows administrator prompt.
2. In Options, enter complete executable filenames (one per line, or comma-separated), save, and enable process proxy. Same-named processes are all selected; differently named child processes must be listed separately.
3. Connect. The application copies the driver to the protected Windows driver directory and starts **`sgfg_sockfilter`**, a demand-start kernel service owned by SGFG. No manual installation script is needed.
4. Restart target applications or create new connections. Existing sockets are not guaranteed to be reclassified.
5. Disconnect or exit. Filtering is detached first, then the driver is stopped, the service deleted, and the installed driver file removed. Packaged runtime assets stay beside the executable for the next connection.

Only the SGFG-owned service/file is managed. An existing vendor service named `sockfilter`, Proxifier services, and other drivers are not overwritten or removed. A cross-process guard prevents two SGFG instances from managing the same driver simultaneously. Startup failure rolls back newly installed resources; after abnormal termination, the next process-proxy connection attempts recovery of its own stale driver. A blocked unload/deletion is reported rather than claimed successful, and may require closing other tools or restarting Windows. The application never changes Secure Boot, memory integrity, signature enforcement or test-signing settings.

DNS scope is limited to captured queries from selected processes. Windows DNS Client service traffic (usually `svchost.exe`), DoH/DoT, and ordinary HTTP/SOCKS5 domain resolution are not globally rewritten. The fixed public DNS resolver cannot resolve private enterprise-only names. This feature is not a kill switch; disconnecting or crashing may allow new direct connections. Options currently remain in memory for the application session.

### Driver requirements and safety

- **License and signature:** NetFilter SDK is a commercial dependency, not covered by this project's MIT-0 license. Obtain a vendor license that permits your intended use and redistribution, plus a driver signed for the target Windows environment. The SDK supplied for internal evaluation is test-signed; Secure Boot, memory integrity or code-integrity policy may reject it. Do not treat an evaluation build as production-ready merely because the application UI is release-style.
- **Evaluation limits:** the vendor documents limits on filtered TCP connections and UDP sockets. Once exhausted, filtering may require a Windows restart to work again. There is no documented reliable remaining-quota API used by this application; reconnecting or reinstalling the driver is not a guaranteed reset. The application cannot guarantee detection of quota exhaustion or prevent direct traffic after driver failure.
- **Owned resources:** the application manages only service `sgfg_sockfilter` and `%SystemRoot%\System32\drivers\sgfg_sockfilter.sys`. The service is demand-start, not boot-start. It validates an existing service's type, display name and driver path before recovery, and refuses to overwrite a different driver file at the reserved path. Old manual installations under service name `sockfilter` are left untouched; the SDK's original uninstall script does not remove the new SGFG-owned service.
- **Cleanup failures:** disconnect/exit attempts unload and removal after releasing filtering. If stopping fails, the application does not proceed to delete the driver service/file. Abnormal termination cannot guarantee immediate cleanup; the next process-proxy connection attempts recovery. Do not delete driver files while they are loaded, or remove services belonging to other software.
- **No security-policy changes:** the application does not disable Secure Boot, memory integrity or signature enforcement, enable test-signing, or automatically reboot. Prefer an isolated evaluation machine or a properly signed, licensed vendor driver.

### Traffic boundaries

- Matching uses complete case-insensitive executable basenames, not paths, PIDs or wildcards. Unselected processes and the proxy application's own PID bypass filtering. This does not stop applications or LAN clients from explicitly connecting to an exposed local proxy or SS relay.
- Loopback, link-local, multicast and broadcast traffic normally bypass filtering. Captured DNS port 53 is an exception, including queries originally sent to a router or loopback resolver; private unicast destinations otherwise remain proxied.
- Captured UDP DNS is converted to TCP through the selected node, with the original DNS peer and query ID restored in replies. Oversized replies are truncated with the DNS TC flag for TCP retry; query errors return SERVFAIL, with no local DNS fallback. Do not add the shared Windows host `svchost.exe` merely to capture system DNS.
- WebTransport forwards application TCP only: captured DNS is converted to TCP and other non-local UDP is dropped, not sent directly. Hysteria2/Turtleor support ordinary UDP forwarding. Failed forwarding handshakes or full queues do not trigger a direct fallback while traffic is captured, but this is **not a kill switch**.
- IPv6 TCP and unconnected UDP forwarding are supported. If Windows has no usable IPv6 route, an application's connected UDP socket may fail before sending; this implementation does not rewrite UDP `connect` to supply a missing route.

### Driver troubleshooting

1. Check that the complete runtime directory is beside the executable, the DLL is the x64 C API build, and the application was launched with administrator privileges.
2. During an active process-proxy connection, `sc.exe query sgfg_sockfilter` should report `RUNNING`. After a successful disconnect, querying the service should report that it does not exist. Ordinary proxy mode does not create this service.
3. For a driver-load error, retain the Windows error shown in the application and check **Event Viewer → Applications and Services Logs → Microsoft → Windows → CodeIntegrity → Operational**. Resolve signing or licensing with the vendor rather than disabling system protections.
4. For unload/deletion errors, close other SGFG instances and service-management tools that may hold handles, then retry. A restart may be needed if Windows cannot unload the driver. Do not use the vendor's generic uninstall script against unrelated installations.
5. Avoid overlapping Proxifier/other proxy rules: configure the actual SGFG executable and selected applications as Direct in those tools where appropriate. Detecting another proxy process does not itself prove a conflict; SGFG does not change its rules or stop its services.
6. If traffic still fails, record the application error, Windows version, selected transport and executable names, and whether TCP, UDP or IPv6 is affected. Detailed process-flow logs and counters can be enabled before launch with `SGFG_SOCKFILTER_DEBUG=1`; leave this unset for normal use. Logs may contain process names and destination addresses, so review them before sharing.

Ordinary automated tests do not install or load the driver. The v11.3 lifecycle rollback/order checks use fake backends; these do not replace an elevated connect/disconnect test on the target Windows system. Real driver and external-node probes are explicitly opt-in.

## Server Deployment

The following is a reference deployment using **GoST** and **Hysteria2** on Linux (Debian/Ubuntu shown). A custom compatible WebTransport server can replace GoST; these examples do not describe the internal node implementation. You need a domain name per node pointing at the server and a valid TLS certificate.

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
3. For ordinary proxy use, distribute `bin/sgfg-ng.exe`. For process proxying, also distribute the licensed `sockfilter/` runtime directory beside it. Custom installers and cross-build pipelines must preserve this directory layout; packaging a driver is not the same as installing it.
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
│   ├── sockfilter/             # process rules, DNS-over-TCP, native API and managed driver lifecycle
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
- Process proxying currently supports Windows x64 only. Driver loading depends on Windows signature/security policy; the SDK evaluation driver is not a production distribution license.
- WebTransport application forwarding is TCP-only. Hysteria2/Turtleor retain UDP support. Captured DNS 53 queries use TCP regardless of the selected transport.
- Conflicting third-party proxy rules may cause repeated proxying; exclude SGFG and the selected applications from those rules where appropriate.
- Driver installation/unload regression tests use fake lifecycle backends by default. Real driver/network tests are opt-in, require elevation and must not run concurrently with the application using the driver.
- The Hysteria2 local SOCKS5 UDP ASSOCIATE implementation is simplified: fragmented (FRAG != 0) packets are dropped per RFC intent; the UDP relay socket binds to the local address of its TCP control connection (works for LAN clients when LAN access is enabled).
- Local connections are capped at 512 concurrent sessions; HTTP CONNECT headers are capped at 64 KB.
- Node information is hardcoded by design; changing nodes requires editing `internal/core/nodes.go` and rebuilding.

## Acknowledgments

This project was built in collaboration with AI pair-programming and review models:

- **Kimi K3** — co-developer: full Go rewrite, backend architecture, UI implementation, debugging, and documentation
- **GPT 6 Astra** — co-developer: SockFilter process-proxy integration, DNS-over-TCP forwarding, driver lifecycle management, settings-save fixes, regression tests, and v11.3 release documentation
- **GPT 5.6 Luna** — code review
- **Grok 4.5** — code review
- **Grok 4.6** — code review
- **Muse Spark 1.3** — code review

## License

This project's own code is released under the **MIT-0 (MIT No Attribution)** license — see [LICENSE](LICENSE). You may use it freely, with no attribution required.

**Important:** the bundled dependencies [GoST](https://github.com/go-gost/gost) and [Hysteria2](https://github.com/apernet/hysteria) are licensed under the **MIT license**, which *does* require retaining their copyright and license notices. When redistributing this software (source or binary), you must include the [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES) file shipped in this repository.
