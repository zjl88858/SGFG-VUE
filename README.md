# SGFG-VUE

A proxy client for Windows, written in Go. [oh-my-hysteria](https://github.com/zjl88858/oh-my-hysteria) is compiled into the desktop user's GUI process as a Go library, using COPA congestion control and complete Blink obfuscation. GOST remains only for the optional local Shadowsocks LAN entrance, not remote-node transport. Optional process proxying uses a separate elevated driver helper, NetFilter SDK runtime and kernel driver. The UI is built with Wails v3 + Vue 3 + TypeScript in a pale-green and pink candy theme.

**Current version: v12.1**

### What's new in v12.1

- Support direct Administrator logon and already-elevated desktops: an elevated GUI must match the current desktop Shell's user SID and Windows session ID, and the Shell must itself be elevated. Administrator Protection's isolated account and separate elevation above an ordinary desktop remain blocked.
- Refine the pale-green/pink UI with an SGFG wordmark and version capsule, horizontal transport choices, a combined connection/traffic card, a fixed-position connection action bar, and three settings sections.
- Fix the animated turtle's head being clipped at either end of its track.
- Update both the GUI and driver helper to v12.1 (Windows file version `12.1.0.0`). This release is staged in `bin/v12.1/`; distribute the matching helper, `sockfilter/` and `licenses/` alongside the GUI.

### What's new in v12.0

- Replace GOST WebTransport and the official Hysteria client with a pinned OMHY core/extras snapshot and its required QUIC fork.
- Provide exactly two modes, **omhy-direct** (UDP 10520–10522) and **omhy-turtleor** (UDP 11520–11522), with two nodes, **argon** and **barium**. Both modes use COPA and Blink and forward application TCP/UDP.
- Use all Blink features from upstream: authenticated framing, padding, initial junk packets, replay/resource limits, fragmentation/reassembly and the 1200-byte inner QUIC packet contract. The supplied profiles use 1150-byte outer packets and fragmentation.
- Preserve v11.4's ordinary-user GUI/elevated helper separation, process DNS, UDP endpoint pinning, local encrypted preferences and per-connection traffic counters.

> ## Server Configuration and Licensing
>
> All deployment-specific node addresses, credentials and per-mode Blink secrets are defined in **`internal/core/nodes.go`**. Sanitize that single file before publishing source. `config_demo/` is private reference material only: the application neither imports nor embeds it, and the entire directory can be deleted before publication without affecting the build. The values in this README are placeholders, not a provided proxy service. Hardcoded node secrets remain recoverable from the compiled GUI; the SS-password encryption feature does not protect them.
>
> This software is a **client only**. Deploy and operate your own compatible OMHY servers and configure `internal/core/nodes.go` before building (see [Node Configuration](#node-configuration)). The authors do not provide, sell, or endorse any proxy service.

## Features

| Feature | Description |
|---|---|
| omhy-direct | OMHY + COPA + Blink, direct server egress, entry UDP ports 10520–10522 |
| omhy-turtleor | OMHY + COPA + Blink, Turtleor server-side egress, entry UDP ports 11520–11522 |
| Node selection | Random selection per transport across configured nodes, with ICMP latency probing and automatic failover |
| Local proxy | HTTP or SOCKS5 (mutually exclusive), listening on 127.0.0.1 or 0.0.0.0 (LAN access), default port 7788 with automatic increment if occupied |
| System proxy | Automatically sets/restores the Windows system proxy on connect/disconnect; detects and repairs stale settings after abnormal exits |
| Process proxy | Windows x64: proxy selected `.exe` names; other processes bypass. Case-insensitive exact filenames, including future instances. Mutually exclusive with the system proxy |
| Driver lifecycle | Connecting with process proxy enabled launches a separate elevated headless helper, requesting UAC when needed. It manages the application-owned demand-start driver and filtering, then unloads/removes the driver on disconnect. The GUI and remote proxy core retain the desktop user's identity and normally run unelevated; already-elevated desktops are supported |
| Process DNS | Captured UDP/TCP port 53 requests use `tcp://1.1.1.1:53` through the selected node; UDP requests are translated to TCP and replies restored to the original DNS peer |
| Options window | Separate Wails window for proxy options, custom port, and the SS relay |
| SS LAN relay | After connecting, exposes a `ss://aes-128-gcm` entrance forwarding to the local proxy, so other LAN devices can share the connection |
| Password generator | Random relay passwords (8-16 alphanumeric characters) |
| Traffic usage | Live upload/download totals for the current OMHY connection, including process proxy and SS relay traffic, counted once |
| Local preferences | Proxy switches, process list, requested local port, and SS relay port/password survive application restarts; node selection and traffic totals are never saved |

## Traffic usage and local preferences (v11.3.1)

### Upload and download

The main window displays separate **upload** (application → node) and **download** (node → application) totals, refreshed approximately once per second in B/KiB/MiB/GiB units. A successful new proxy connection starts fresh counters; disconnecting freezes the last totals for viewing. Restarting the application resets them to zero. A failed connection attempt does not erase the preceding session's display.

Counting occurs at the OMHY client's TCP stream and UDP payload interface in both modes. Process-proxy traffic, captured DNS forwarded through the node, explicit local proxy clients and SS relay clients share this boundary. The SS-to-local-proxy forwarding stage is not counted again, so relayed traffic contributes once. Bypassed traffic, node-selection probes, proxy authentication, QUIC/Blink framing, padding, junk packets and transport retransmissions are excluded. Application protocol headers (for example HTTP and DNS-over-TCP framing) are included. These are application-level transfer counters, not a network-adapter or ISP billing measurement, and an accepted write does not prove delivery to the final server.

### Saved settings

Settings are stored in **`%APPDATA%\SGFG\settings.json`** on Windows (Go's `os.UserConfigDir()` for the account running the application). Moving/upgrading the executable does not change this location. The file is created on the first successful settings save; an absent file uses the default options and relay port 8388.

| Saved locally | Restored behavior |
|---|---|
| System proxy, allow LAN access, use SOCKS5 | Restore the three proxy-option switches; apply when connecting |
| Process proxy enabled and executable list | Restore the mode and normalized exact filenames; process mode uses SOCKS5 and disables the system-proxy option |
| Local listening port | Restore the requested port; 0 means default 7788. An automatically incremented runtime port is not saved over the requested value |
| SS relay port and password | Restore the fields, including a generated password, without automatically enabling the relay |

Transport/node selection, connection/relay activation, latency and upload/download totals remain runtime-only. Startup does not connect or select a node automatically.

Proxy switches and committed local-port edits save automatically; use **Save (保存)** for process-list edits. SS port/password edits save on field commit, password generation, **Save (保存)** in the relay section, or before enabling the relay. Pending or failed saves retain the draft and are shown in the options window; closing via its title bar checks for unsaved changes. Proxy options require disconnecting before editing, while relay settings can be changed with the main proxy connected as long as the relay is stopped.

The backend validates preferences and writes a same-directory temporary file before replacing the configuration. Failed writes leave the previous in-memory settings and file intact. Malformed, unsupported or unreadable settings produce a visible startup error and use defaults without silently replacing the original file; repair or move the file and restart before saving again. To reset preferences, exit the application and remove this file.

**The SS password is encrypted locally with AES-128-GCM.** A 128-bit key is derived from SHA-256 of an application-specific purpose string, Windows `MachineGuid` and the current account's SID. Each save uses a fresh cryptographically random 96-bit nonce and a 128-bit authentication tag; the JSON stores a versioned Base64 envelope in `relay.passwordEncrypted`, never the key or plaintext password. Other preferences remain ordinary JSON. The password is decrypted in memory for the options window and SS relay; neither the password nor the machine/account identifiers are uploaded.

This is lightweight machine/account-bound protection, not a credential vault: these identifiers are not secrets, and software able to read them and the configuration can reproduce the key. Copying the configuration to another machine/account, reinstalling Windows, or changing those identifiers prevents normal decryption. Moving or updating the executable on the same installation/account does not. The GUI must run as the interactive user; the elevated helper never reads these settings or decrypts the SS password. Configuration previously saved under a different or temporary administrator account is not automatically imported.

The settings schema is now **version 2**. On startup, valid version-1 plaintext settings are automatically encrypted and replaced using the same temporary-file save mechanism, without losing other preferences. A failure to obtain the machine identity, encrypt, write, or authenticate ciphertext is reported; there is no plaintext fallback and no silent replacement of the original file. Repair or move an unreadable configuration and restart, then re-enter the relay password. An empty saved password needs no machine identity. Earlier plaintext backups are not rewritten by this migration.

## Tech Stack

- **Backend**: Go 1.26+ (preferred toolchain 1.26.8)
  - `github.com/zjl88858/oh-my-hysteria/core/v2` + `extras/v2` — OMHY client + full Blink wrapper
  - OMHY's local `github.com/apernet/quic-go` fork — required transport hooks; not the stock module
  - `github.com/go-gost/x` — local SS relay only, with no WebTransport dialer registered
  - `github.com/wailsapp/wails/v3` — desktop application framework
  - `golang.org/x/sys` — Windows registry access
- **Frontend**: Vue 3 + TypeScript + Vite, Bun as package manager, `@wailsio/runtime`
- **Optional process filtering**: NetFilter SDK SockFilter C API (Windows x64). This is a **commercial, separately licensed dependency**, not an open-source component of SGFG. Obtain appropriate runtime/driver distribution rights from the vendor.

> OMHY is pinned at commit **`22b2f02784138663c13e1a0e9e0a06c5dc202dbf`** (v1.0.0-20261029) under `third_party/oh-my-hysteria/`. Root `replace` directives select the bundled core, extras and QUIC fork. Preserve that directory, including all nested third-party code and licenses. Do not replace just the transport with stock quic-go or update the snapshot independently of server compatibility tests.

## Node Configuration

Edit the `Nodes` table and shared `nodeBlink` defaults in `internal/core/nodes.go`. No YAML file is read or embedded for node configuration. For example (replace placeholders):

```go
var Nodes = []Node{
    {
        Name: "argon",
        Host: "argon.example.com",
        Password: "REPLACE_NODE_AUTH",
        DirectBlink: nodeBlink("REPLACE_DIRECT_BLINK_SECRET_32_BYTES_OR_MORE"),
        TurtleorBlink: nodeBlink("REPLACE_TURTLEOR_BLINK_SECRET_32_BYTES_OR_MORE"),
    },
    {
        Name: "barium",
        Host: "barium.example.com",
        Password: "REPLACE_OTHER_NODE_AUTH",
        DirectBlink: nodeBlink("REPLACE_OTHER_DIRECT_BLINK_SECRET_32_BYTES_OR_MORE"),
        TurtleorBlink: nodeBlink("REPLACE_OTHER_TURTLEOR_BLINK_SECRET_32_BYTES_OR_MORE"),
    },
}
```

A node's modes share its host and auth but have independent Blink passwords. The shared `nodeBlink` function, also in `nodes.go`, sets 1150-byte outer packets, fragmentation on, padding up to 16 bytes, three initial junk packets and 3072 receive sessions. The remaining budgets use upstream defaults; edit them there or use an explicit per-node `BlinkConfig` if needed. General option mapping and validation live in `internal/core/blink.go`, which contains no deployment secrets.

Every upstream Blink option is supported through the Go fields `Password`, `MaxPacketSize`, `PaddingMax`, `JunkPackets`, `Fragmentation`, `MaxSessions`, `MaxReassemblyBytes` and `VerifyPacketsPerSecond`. Nil padding/junk pointers select defaults; pointers to zero disable them. Invalid Blink options and out-of-mode ports are rejected without logging credentials. The client uses COPA and lazy connection establishment; saved GUI options determine local protocol, address and port.

**Blink does not support in-session port hopping.** Node selection chooses one port from the mode's inclusive three-port range; a later reconnect may select another single port. The same Blink socket never changes its remote endpoint. The UI's port is the initial selected entry port; reconnection logs report the actual connected endpoint. The mode names, ranges and local-port defaults are also in `nodes.go`. Rebuild after editing this file. The reference-only `config_demo/` directory is never needed to build, test or run SGFG.

Turtleor is an OMHY server-side egress variant; the Windows client does not create a WARP tunnel. Its server must route outbound traffic through the intended Turtleor/WARP service. Simply changing the client mode does not configure that server egress.

## Build Requirements

| Requirement | Version |
|---|---|
| OS | Windows 10/11 (sysproxy and ICMP ping are Windows-only implementations) |
| Go | 1.26+; preferred toolchain 1.26.8 (per `go.mod`) |
| Bun | 1.x, available in `PATH` |
| Wails CLI | `v3.0.0-alpha2.117`, compiled using Go 1.26+; an older CLI binary built with Go 1.25 cannot fully type-check the new core when generating bindings |
| WebView2 Runtime | required at runtime (preinstalled on most Win10/11 systems) |
| Administrator privileges | Launch the GUI normally, not as administrator. Only the separate process-filter helper requests UAC when connecting in process-proxy mode; UAC/security policies are not bypassed |
| NetFilter SDK (optional) | Compatible x64 SockFilter C API DLL and a driver whose signature is accepted by the target Windows system |

## Build Commands

```powershell
# install frontend dependencies (first time only)
cd frontend; bun install; cd ..

# production build: generates bindings, builds the frontend with bun,
# generates icons/syso, and compiles the exe
wails3 build
# output: bin/sgfg-ng.exe, bin/sgfg-driver-helper.exe (x64), and bin/sockfilter/ when SDK assets are available
# also distribute bin/licenses/ (OMHY, QUIC and controller license notices)

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
  sgfg-driver-helper.exe         # elevated, headless; keep beside the GUI
  sockfilter/
    nfapi.dll                    # sockfilter/bin/release_c_api/x64/nfapi.dll
    driver/x64/sockfilter.sys     # sockfilter/bin/driver/x64/sockfilter.sys
    license.rtf
  licenses/                      # staged by build/windows/stage-core-licenses.ps1
```

`build/windows/stage-sockfilter.ps1` copies assets only; it never installs a driver. Without SDK files, the ordinary proxy build remains available and process proxying reports the missing dependency when enabled. Do not commit or redistribute vendor binaries/source unless your license permits it. The bundled SDK used for internal evaluation has connection limits and test-signing restrictions; preparing a production UI does not turn an evaluation driver into a production-licensed driver.

Use the **x64 C API** `nfapi.dll`, not the SDK's C++ or Win32 DLL. Only the elevated helper loads it, from `sockfilter/nfapi.dll` beside the helper executable; dependent DLL lookup is restricted to that directory and Windows System32, not `PATH` or the working directory. Both executables are built in Go without CGO; `sgfg-driver-helper.exe` is now a required separate component for process proxying. The separate internal `README.sockfilter.md` is optional and need not be published: this README contains the essential deployment and safety information. The staging script copies that extra document only when present.

The native Windows task builds the helper automatically using `build/windows/build-driver-helper.ps1`. For a manual build, run `powershell -NoProfile -File build/windows/build-driver-helper.ps1 -OutputDirectory bin`. This generates the helper's own administrator manifest and builds `./cmd/sgfg-driver-helper`; it does not install or run the driver. The GUI manifest uses `asInvoker`, the helper uses `requireAdministrator`. Keep the GUI and helper from the same build. Cross-build/installer pipelines must build and package the helper explicitly rather than assuming a GUI-only build includes it. Obfuscating the GUI does not obfuscate the separately built helper.

### Using process proxy

1. Launch the application normally from your desktop. On an ordinary desktop, do not separately use **Run as administrator**, an elevated terminal, or an administrator compatibility shortcut for the GUI. Direct Administrator logon (including an already-elevated desktop with UAC disabled) is supported when the GUI uses the same account and Windows session as that desktop.
2. In Options, enter complete executable filenames (one per line, or comma-separated), save, and enable process proxy. Same-named processes are all selected; differently named child processes must be listed separately.
3. Connect and approve UAC for **`sgfg-driver-helper.exe`**. The helper copies the driver to the protected Windows driver directory and starts **`sgfg_sockfilter`**, a demand-start kernel service owned by SGFG. It owns the SDK callbacks and local transparent TCP/UDP/DNS forwarding; the node-facing proxy core remains in the ordinary-user GUI. No manual installation script is needed. Cancelling UAC rolls back this connection attempt without changing Windows security settings.
4. Restart target applications or create new connections. Existing sockets are not guaranteed to be reclassified.
5. Disconnect or exit. Filtering is detached first, then the driver is stopped, the service deleted, and the installed driver file removed. Packaged runtime assets stay beside the executable for the next connection.

### Privilege separation and Windows Administrator Protection

The GUI, normally unelevated, exclusively owns Wails/WebView2, `%APPDATA%` preferences and password decryption, HKCU system-proxy changes, the remote OMHY client and traffic totals. Before accessing settings, the system proxy or WebView2, startup checks the process token. An unelevated GUI is allowed directly. An elevated GUI is allowed only if its user SID and Windows session ID match the current desktop Shell's token and that Shell is itself elevated. This supports direct Administrator logon and already-elevated desktops without writing another user's profile or HKCU. Under Administrator Protection the isolated elevated account has a different SID and is rejected; separate same-account elevation above an unelevated desktop is also rejected.

The desktop is located with `GetShellWindow`, not by enumerating `explorer.exe` or using the active console session, so a remote desktop uses its own Shell. No account-name prefixes, localized Administrator names, OS-version assumptions or security-policy changes are involved. If an elevated launch cannot read a stable desktop identity (for example, the Shell is restarting or absent), startup reports that verification failure and asks the user to retry after the desktop is ready. These checks keep the GUI attached to the intended user profile; they do not identify an arbitrary account as an Administrator Protection account solely from its name. An ordinary HTTP/SOCKS5 or system-proxy connection does not launch a helper or show UAC. Merely saving/enabling the process-proxy option does not elevate either; connecting does.

Each process-proxy connection launches one helper through `ShellExecuteExW` with the `runas` verb, not a command interpreter. The helper has no GUI/WebView2, no node credentials, no SS credentials, no user-settings access and no HKCU proxy logic. It talks only to the parent's loopback SOCKS5 listener. Both the GUI PID and helper PID bypass filtering to prevent recursion even if their filenames appear in the process list.

Control uses a randomly named, first-instance **local named pipe**, rejecting remote clients, with a DACL for the launching user, administrators and SYSTEM. The GUI checks the connecting helper's kernel-reported PID against the specific process handle returned by UAC launch. The helper checks the pipe server PID against its pinned parent, verifies that the executables are in the same resolved directory and Windows session, and watches the original parent process handle. Different elevated user SIDs are deliberately permitted; a same-user-SID check would break Administrator Protection. The size-bounded, versioned protocol accepts only one validated start request, heartbeat and stop; it does not expose arbitrary commands, paths, DLLs, driver names or general service-management operations.

On disconnect, the GUI waits for a cleanup acknowledgment before closing the node-facing proxy. A dead GUI process, broken pipe or missing heartbeat makes the helper attempt SDK/driver cleanup and exit. If the helper dies unexpectedly, the GUI marks process filtering inactive and disconnects the local proxy rather than continuing to report a filtered connection. Forced helper termination or an OS failure cannot guarantee driver removal; owned-driver recovery on the next connection still applies. This remains **not a kill switch**.

Use a trusted distribution directory and preferably sign both executables; an installed deployment should protect the helper, DLL and driver assets with appropriate ACLs. This is a per-connection, user-consented UAC helper, not a permanently installed privileged service or a substitute for secure package distribution. Existing driver signing/evaluation restrictions still apply.

Only the SGFG-owned service/file is managed. An existing vendor service named `sockfilter`, Proxifier services, and other drivers are not overwritten or removed. A cross-process guard prevents two SGFG instances from managing the same driver simultaneously. Startup failure rolls back newly installed resources; after abnormal termination, the next process-proxy connection attempts recovery of its own stale driver. A blocked unload/deletion is reported rather than claimed successful, and may require closing other tools or restarting Windows. The application never changes Secure Boot, memory integrity, signature enforcement or test-signing settings.

DNS scope is limited to captured queries from selected processes. Windows DNS Client service traffic (usually `svchost.exe`), DoH/DoT, and ordinary HTTP/SOCKS5 domain resolution are not globally rewritten. The fixed public DNS resolver cannot resolve private enterprise-only names. This feature is not a kill switch; disconnecting or crashing may allow new direct connections. Since v11.3.1, preferences are saved locally as described above; connection state and traffic remain in memory only.

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
- Both OMHY modes forward application TCP and UDP. Captured DNS still uses TCP regardless of mode. Failed forwarding handshakes or full queues do not trigger a direct fallback while traffic is captured, but this is **not a kill switch**.
- IPv6 TCP and unconnected UDP forwarding are supported. If Windows has no usable IPv6 route, an application's connected UDP socket may fail before sending; this implementation does not rewrite UDP `connect` to supply a missing route.

### Driver troubleshooting

1. Check that `sgfg-driver-helper.exe` and the complete runtime directory are beside the GUI, the DLL is the x64 C API build, and the GUI was launched **without** administrator privileges. Approve the helper's UAC request only when connecting with process proxy enabled. Remove any old “Run this program as administrator” shortcut/compatibility setting yourself; the app does not change Windows security policies.
2. During an active process-proxy connection, `sc.exe query sgfg_sockfilter` should report `RUNNING`. After a successful disconnect, querying the service should report that it does not exist. Ordinary proxy mode does not create this service.
3. For a driver-load error, retain the Windows error shown in the application and check **Event Viewer → Applications and Services Logs → Microsoft → Windows → CodeIntegrity → Operational**. Resolve signing or licensing with the vendor rather than disabling system protections.
4. For unload/deletion errors, close other SGFG instances and service-management tools that may hold handles, then retry. A restart may be needed if Windows cannot unload the driver. Do not use the vendor's generic uninstall script against unrelated installations.
5. Avoid overlapping Proxifier/other proxy rules: configure the actual SGFG executable, **`sgfg-driver-helper.exe`** and selected applications as Direct in those tools where appropriate. Detecting another proxy process does not itself prove a conflict; SGFG does not change its rules or stop its services.
6. If traffic still fails, record the application error, Windows version, selected transport and executable names, and whether TCP, UDP or IPv6 is affected. Detailed process-flow logs and counters can be enabled before launch with `SGFG_SOCKFILTER_DEBUG=1`; leave this unset for normal use. Logs may contain process names and destination addresses, so review them before sharing.

Ordinary automated tests do not install or load the driver. The v11.3 lifecycle rollback/order checks use fake backends; these do not replace an elevated connect/disconnect test on the target Windows system. Real driver and external-node probes are explicitly opt-in.

## Server Deployment

Use the compatible [OMHY server](https://github.com/zjl88858/oh-my-hysteria), not the retired GOST WebTransport or stock Salamander-only Hysteria setup. A node needs a trusted TLS certificate for its configured hostname. The Windows client verifies TLS normally; it does not enable `insecure`.

Example direct server profile for one entry port:

```yaml
listen: :10520
tls:
  cert: /etc/letsencrypt/live/argon.example.com/fullchain.pem
  key: /etc/letsencrypt/live/argon.example.com/privkey.pem
auth:
  type: password
  password: REPLACE_NODE_AUTH
obfs:
  type: blink
  blink:
    password: REPLACE_WITH_THE_DIRECT_PROFILE_SECRET
    maxPacketSize: 1150
    fragmentation: true
    paddingMax: 16
    junkPackets: 0
    maxSessions: 3072
congestion:
  type: copa
ignoreClientBandwidth: true
```

Serve all three direct ports (10520–10522) and all three Turtleor ports (11520–11522), either with compatible listeners or server-side forwarding to the appropriate mode's OMHY instance. Do not redirect both mode ranges to the same instance if their Blink secrets or egress policies differ. Configure the second instance's Blink password from the Turtleor profile and its server-side outbound policy for Turtleor/WARP egress. Both ends must agree on Blink password and fragmentation. Server `junkPackets` must be zero or omitted; the client examples use three.

Open these **UDP** ranges in the server firewall and provider security group. Certificate issuance may separately require TCP 80/443. COPA configures the sender on each side independently: this client requests COPA for uploads; configure the server for COPA as well if downloads should use it. See the pinned upstream `README.md`, `docs/copa.md` and `docs/blink.md` for parameter limits and experimental-status warnings. This application does not deploy or reconfigure servers.

## Client Build & Deployment

1. Edit `internal/core/nodes.go` with your own servers (see [Node Configuration](#node-configuration)); auth and per-mode Blink secrets must match the server configuration. Before publishing source, sanitize only that file and remove any private `config_demo/` reference directory. No sanitization is performed automatically.
2. Run `wails3 build`.
3. Distribute `bin/sgfg-ng.exe` and `bin/licenses/`. For process proxying, also distribute `bin/sgfg-driver-helper.exe` and the licensed `sockfilter/` runtime directory beside it. Custom installers and cross-build pipelines must preserve this directory layout; packaging a driver is not the same as installing it.
4. On first run, pick a transport, connect, and optionally open the options window (gear icon) for SOCKS5 mode, LAN access, custom port, or the SS relay.

## Project Structure

```
sgfg-ng/
├── main.go                     # Wails entry: windows, service registration, embedded assets
├── service.go                  # ProxyService: methods bound to the frontend
├── build/                      # wails3 build assets (icon, syso, product info)
├── internal/
│   ├── core/
│   │   ├── nodes.go            # sole deployment node/secret table, Blink defaults and port ranges
│   │   ├── blink.go            # full Blink option mapping/validation, no deployment secrets
│   │   ├── gost.go             # local SS LAN entrance only
│   │   ├── omhy.go             # OMHY + COPA + Blink, lazy reconnect and tracked shutdown
│   │   └── localproxy.go       # local SOCKS5 (incl. UDP ASSOCIATE) / HTTP CONNECT server
│   ├── proxy/manager.go        # core state machine: select/connect/disconnect/relay/sysproxy
│   ├── sockfilter/             # process rules, DNS-over-TCP, native API and managed driver lifecycle
│   ├── filterhelper/           # authenticated local IPC and elevated helper lifecycle
│   ├── sysproxy/               # Windows system proxy (registry + wininet refresh)
│   └── ping/                   # ICMP ping via iphlpapi IcmpSendEcho (no admin needed)
├── third_party/oh-my-hysteria/ # pinned upstream core/extras/QUIC source and notices
├── cmd/sgfg-driver-helper/     # headless elevated entry point
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
- COPA and Blink are experimental upstream features, not a guarantee of DPI resistance or availability. Blink has no in-session port hopping; reconnects can choose another entry port. Both modes support TCP/UDP; captured DNS 53 always uses TCP.
- Conflicting third-party proxy rules may cause repeated proxying; exclude SGFG and the selected applications from those rules where appropriate.
- Driver installation/unload regression tests use fake lifecycle backends by default. Real driver/network tests are opt-in, require elevation and must not run concurrently with the application using the driver.
- The OMHY local SOCKS5 UDP relay pins the client's IP and **nonzero UDP source port from the TCP ASSOCIATE request**, before receiving any datagrams. The IP must match the TCP peer (an unspecified IP uses that peer); all other source endpoints, invalid reserved bytes and fragments are dropped. To prevent local first-packet hijacking, zero-port and domain-name source requests are rejected with SOCKS5 REP=2 rather than learned from incoming UDP. The built-in process-proxy helper already declares its real UDP endpoint and is unaffected; external UDP clients must do the same. The relay socket binds to the TCP connection's local address and works for LAN clients when LAN access is enabled. This endpoint binding does not turn the unauthenticated local proxy into a per-user authentication boundary.
- Local connections are capped at 512 concurrent sessions; HTTP CONNECT headers are capped at 64 KB.
- Node settings are hardcoded in `internal/core/nodes.go`; changing them requires editing that one file and rebuilding. `config_demo/` is not a build or runtime dependency. ICMP probing is not proof that UDP authentication or a particular remote destination is reachable. A locally ready lazy proxy is not yet proof of an upstream session.

## Acknowledgments

This project was built in collaboration with AI pair-programming and review models:

- **Kimi K3** — co-developer: full Go rewrite, backend architecture, UI implementation, debugging, and documentation
- **GPT 6 Astra** — co-developer: SockFilter process-proxy integration, DNS-over-TCP forwarding, driver lifecycle management, settings persistence and password encryption, traffic accounting, OMHY/COPA/Blink migration, SOCKS5 UDP endpoint-pinning fix, pale-green/pink UI redesign and animation fixes, desktop-identity startup checks for direct Administrator logon, regression testing, and v11.3–v12.1 release documentation and packaging
- **GPT 6.1 Sol** — co-developer: v11.4 privilege separation, elevated driver helper, authenticated local IPC, lifecycle regression tests, Windows Administrator Protection compatibility work, and release documentation
- **Gemini 4 Carbon** — co-developer: desktop-identity startup checks for direct Administrator logon research, github project management
- **GPT 5.6 Luna** — code review
- **GPT Daybreak Blue** — code review, security review
- **Grok 4.5** — code review
- **Grok 4.6** — code review
- **Muse Spark 1.3** — code review
- **Muse Spark Open** — code review
- **Gemini 4 Argon** — code review, security review, github project management
- **Kimi K3.1 Pro** — code review

## License

This project's own code is released under the **MIT-0 (MIT No Attribution)** license — see [LICENSE](LICENSE). You may use it freely, with no attribution required.

**Important:** SGFG's own license does not relicense its dependencies. OMHY original additions use MIT-0; inherited Hysteria2/QUIC and GOST code retain MIT; the COPA adaptation retains TQUIC's Apache-2.0 obligations, and other upstream adaptations retain their original notices. Retain [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md), the snapshot's original license files, and the staged `licenses/` directory when distributing binaries. NetFilter SDK remains separately licensed commercial software.
