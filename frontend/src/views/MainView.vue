<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { api, onLog, onOptions, onScan, onState } from '../bridge'
import { showToast } from '../toast'
import { formatTraffic } from '../traffic'
import avatarUrl from '../assets/avatar.png'
import CandyButton from '../components/CandyButton.vue'
import ModalDialog from '../components/ModalDialog.vue'
import TitleBar from '../components/TitleBar.vue'
import TurtleRunner from '../components/TurtleRunner.vue'

interface TransportItem {
  key: string
  title: string
}

const transports: TransportItem[] = [
  { key: 'omhy-direct', title: '直连' },
  { key: 'omhy-turtleor', title: 'Turtleor' },
]

const connected = ref(false)
const connecting = ref(false)
const activeTransport = ref('')
const nodeLabel = ref('未选择传输方式')
const nodeName = ref('')
const nodeHost = ref('')
const hopPort = ref(0)
const localPort = ref(7788)
const latency = ref(-1)
const latencyTick = ref(0)
const reachable = ref(true)
const scanning = ref(false)
const scanAttempt = ref(0)
const scanHost = ref('')
const version = ref('')
const traffic = ref({ upload: 0, download: 0 })
const uploadDisplay = computed(() => formatTraffic(traffic.value.upload).split(' '))
const downloadDisplay = computed(() => formatTraffic(traffic.value.download).split(' '))
let trafficTimer: number | undefined
let trafficRequest = 0

async function refreshTraffic() {
  const request = ++trafficRequest
  try {
    const snapshot = await api.traffic()
    if (request === trafficRequest) traffic.value = snapshot
  } catch {}
}

const optSocks5 = ref(false)
const optAllowLan = ref(false)
const optCustomPort = ref(0)
const processProxyActive = ref(false)
const endpoint = computed(() => `${optAllowLan.value ? '0.0.0.0' : '127.0.0.1'}:${connected.value ? localPort.value : optCustomPort.value || 7788}`)
const statusText = computed(() => connecting.value ? (connected.value ? '正在断开' : '正在连接') : connected.value ? '已连接' : scanning.value ? '正在选择节点' : '未连接')

async function copyEndpoint() {
  try {
    await navigator.clipboard.writeText(endpoint.value)
    showToast('ok', '代理地址已复制')
  } catch {
    showToast('error', '复制失败，请手动复制地址')
  }
}

const modal = reactive({ open: false, title: '', lines: [] as string[], danger: false })

interface LogLine {
  id: number
  level: string
  msg: string
  time: string
}
const logs = ref<LogLine[]>([])
let logId = 1

function pushLog(level: string, msg: string) {
  logs.value.unshift({ id: logId++, level, msg, time: new Date().toLocaleTimeString('zh-CN', { hour12: false }) })
  if (logs.value.length > 60) logs.value.pop()
}

function showModal(title: string, lines: string[], danger = false) {
  modal.title = title
  modal.lines = lines
  modal.danger = danger
  modal.open = true
}

async function pickTransport(t: TransportItem) {
  if (connected.value) {
    showToast('warn', '请先断开连接再切换传输方式')
    return
  }
  if (scanning.value || connecting.value) return
  activeTransport.value = t.key
  nodeLabel.value = '正在分配节点...'
  nodeName.value = ''
  nodeHost.value = ''
  reachable.value = true
  hopPort.value = 0
  latency.value = -1
  scanning.value = true
  scanAttempt.value = 0
  scanHost.value = ''
  try {
    const res = await api.selectTransport(t.key)
    if (res.err) {
      nodeLabel.value = '未选择传输方式'
      activeTransport.value = ''
      showToast('error', res.err)
      return
    }
    nodeLabel.value = res.nodeLabel
    nodeName.value = res.nodeName
    nodeHost.value = res.host
    hopPort.value = res.port
    latency.value = res.latency
    reachable.value = res.reachable
    latencyTick.value++
    if (!res.reachable) {
      showToast('error', '该节点似乎不可用', 1500)
      showModal('错误', [
        '该节点似乎不可用，你可以尝试以下操作',
        `1. 确认系统 DNS 可以正确解析节点域名: nslookup ${res.host}`,
        '2. 重新点击传输选项，会重新分配节点',
        '3. 如果你当前网络无法使用 ICMP 协议，可以无视此错误',
      ])
    }
  } catch (e: any) {
    nodeLabel.value = '未选择传输方式'
    activeTransport.value = ''
    showToast('error', String(e))
  } finally {
    scanning.value = false
  }
}

async function toggleConnect() {
  if (connecting.value) return
  if (!connected.value) {
    if (!activeTransport.value) {
      showToast('error', '你好像没有选择传输方式哦')
      return
    }
    connecting.value = true
    try {
      const err = await api.connect()
      if (err) {
        showToast('error', err, 3000)
      } else {
        connected.value = true
      }
    } catch (e: unknown) {
      showToast('error', String(e), 3000)
    } finally {
      connecting.value = false
    }
  } else {
    connecting.value = true
    try {
      const err = await api.disconnect()
      if (err) {
        showToast('error', err)
      } else {
        connected.value = false
      }
    } catch (e: unknown) {
      showToast('error', String(e), 3000)
    } finally {
      connecting.value = false
    }
  }
}

function openTelegram() {
  window.open('https://t.me/sgfangun', '_blank')
}

function openOptions() {
  api.openOptions()
}

let pingTimer: number | undefined

async function refreshLatency() {
  if (!nodeName.value) return
  try {
    const res = await api.pingSelected()
    if (res.err) return
    reachable.value = res.reachable
    latency.value = res.latency
    latencyTick.value++
  } catch {}
}

onMounted(async () => {
  pingTimer = window.setInterval(refreshLatency, 10000)
  trafficTimer = window.setInterval(refreshTraffic, 1000)
  void refreshTraffic()
  onScan((e) => {
    if (scanning.value && scanHost.value && e.host && e.host !== scanHost.value) {
      showToast('warn', '节点无响应，自动切换节点')
    }
    scanAttempt.value = e.attempt
    scanHost.value = e.host
  })
  onLog((e) => pushLog(e.level, e.msg))
  onOptions((o) => {
    optSocks5.value = o.useSocks5
    optAllowLan.value = o.allowLan
    optCustomPort.value = o.localPort
  })
  onState((s) => {
    void refreshTraffic()
    connected.value = s.state === 'connected'
    processProxyActive.value = s.processProxy
    if (s.localPort) localPort.value = s.localPort
    if (s.nodeLabel && connected.value) {
      nodeLabel.value = s.nodeLabel
      nodeName.value = s.nodeName
      hopPort.value = s.port
    }
  })
  try {
    version.value = await api.version()
  } catch {}
  try {
    const error = await api.settingsError()
    if (error) {
      pushLog('error', error)
      showModal('本地设置读取失败', [error, '当前使用默认设置，原文件未覆盖。修复后请重新启动应用。'], true)
    }
  } catch {}
  try {
    const o = await api.options()
    optSocks5.value = o.useSocks5
    optAllowLan.value = o.allowLan
    optCustomPort.value = o.localPort
  } catch {}
  try {
    const s = await api.status()
    connected.value = s.state === 'connected'
    processProxyActive.value = s.processProxy
    if (s.localPort) localPort.value = s.localPort
    if (s.transport) {
      activeTransport.value = s.transport
      nodeLabel.value = s.nodeLabel
      nodeName.value = s.nodeName
      nodeHost.value = s.host
      hopPort.value = s.port
    }
  } catch {}
})

onUnmounted(() => {
  if (pingTimer !== undefined) window.clearInterval(pingTimer)
  if (trafficTimer !== undefined) window.clearInterval(trafficTimer)
  trafficRequest++
})
</script>

<template>
  <div class="win">
    <TitleBar title="SGFG" :version="version" show-min show-max @close="Window.Close()" />
    <main class="dashboard">
      <header class="dashboard-header">
        <div class="brand">
          <img class="brand-logo" :src="avatarUrl" alt="" />
          <h1>神龟翻滚</h1>
        </div>
        <div class="header-trail" aria-hidden="true"><TurtleRunner /></div>
        <button class="settings-btn" title="偏好设置" aria-label="偏好设置" @click="openOptions">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3" fill="currentColor" stroke="none"/><circle cx="15" cy="17" r="3" fill="currentColor" stroke="none"/></svg>
        </button>
      </header>

      <section class="transport-section" aria-label="传输方式">
        <div class="transport-grid">
          <button v-for="(t, index) in transports" :key="t.key" class="transport-card"
            :class="{ selected: activeTransport === t.key, turtleor: index === 1 }"
            :aria-pressed="activeTransport === t.key" :aria-busy="scanning && activeTransport === t.key" :title="scanning && activeTransport === t.key ? `${scanHost} · ${scanAttempt}` : t.key" :disabled="connected || connecting || scanning" @click="pickTransport(t)">
            <span class="transport-icon" aria-hidden="true">
              <svg v-if="index === 0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m13 3-9 11h7l-1 7 10-12h-7l1-6Z"/></svg>
              <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 16a8 8 0 0 1 16 0H4ZM8 16l-2 3m10-3 2 3M12 8v8M5 12h14M20 14h1a2 2 0 0 1 0 4h-1M2 16h2"/></svg>
            </span>
            <span class="transport-copy"><strong>{{ t.title }}</strong></span>
            <span class="selection-mark" :class="{ searching: scanning && activeTransport === t.key }" aria-hidden="true"><svg v-if="activeTransport === t.key && !scanning" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><path d="m4 8 3 3 5-6"/></svg></span>
          </button>
        </div>
      </section>

      <div class="overview card">
        <section class="connection-card" aria-label="连接信息">
            <div class="connection-summary">
              <h2 aria-live="polite"><span class="status-dot" :class="{ on: connected }"></span>{{ statusText }}</h2>
              <span v-if="connected && processProxyActive" class="process-badge">进程代理</span>
            </div>
          <div class="node-details">
            <div><span class="detail-label">节点</span><strong :title="nodeHost || nodeLabel">{{ nodeName || '—' }}</strong></div>
            <div><span class="detail-label">端口</span><strong>{{ hopPort || '—' }}</strong></div>
            <div><span class="detail-label" title="ICMP 延迟">延迟</span><strong :key="latencyTick" :class="{ timeout: !reachable }">{{ !reachable ? '超时' : latency >= 0 ? latency : '—' }}<small v-if="reachable && latency >= 0"> ms</small></strong></div>
          </div>
        </section>

        <section class="traffic-card" aria-labelledby="traffic-heading" title="累计代理流量，下次连接清零">
          <div class="section-heading"><h2 id="traffic-heading">流量</h2></div>
          <div class="traffic-values">
            <div class="traffic-item"><span class="traffic-label"><span class="traffic-arrow upload">↗</span>上传</span><strong>{{ uploadDisplay[0] }}<small>{{ uploadDisplay[1] }}</small></strong></div>
            <div class="traffic-item"><span class="traffic-label"><span class="traffic-arrow download">↙</span>下载</span><strong>{{ downloadDisplay[0] }}<small>{{ downloadDisplay[1] }}</small></strong></div>
          </div>
        </section>

        <div class="connection-actions" role="group" aria-label="连接操作">
          <div class="endpoint-bar">
            <span class="protocol-tag">{{ optSocks5 ? 'SOCKS5' : 'HTTP' }}</span>
            <button class="copy-address" title="复制代理地址" aria-label="复制代理地址" @click="copyEndpoint">{{ endpoint }}<svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="5" y="5" width="8" height="8" rx="2"/><path d="M10 3V2H2v8h1"/></svg></button>
          </div>
          <CandyButton class="connect-btn" :variant="connected ? 'danger' : 'primary'" :loading="connecting"
            :disabled="!activeTransport || scanning" :aria-label="connecting ? statusText : connected ? '断开连接' : '建立连接'" @click="toggleConnect">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M12 3v9M6.3 5.8a8 8 0 1 0 11.4 0"/></svg>
            {{ connected ? '断开连接' : '建立连接' }}
          </CandyButton>
        </div>
      </div>

      <section class="card log-panel" aria-labelledby="log-heading">
        <div class="log-heading"><div class="section-heading"><h2 id="log-heading">日志</h2></div><button class="text-button" :disabled="!logs.length" @click="logs = []">清空</button></div>
        <div class="log-list" tabindex="0" aria-label="运行日志列表">
          <div v-for="l in logs" :key="l.id" class="log-line" :class="l.level"><time>{{ l.time }}</time><span class="log-message"><span v-if="l.level === 'error' || l.level === 'warn'" class="log-level">{{ l.level === 'error' ? '错误' : '提示' }} · </span>{{ l.msg }}</span></div>
          <div v-if="!logs.length" class="log-empty">暂无日志</div>
        </div>
      </section>
      <footer class="dashboard-footer"><button class="community-link" @click="openTelegram">Telegram ↗</button></footer>
    </main>

    <ModalDialog :open="modal.open" :title="modal.title" :lines="modal.lines" :danger="modal.danger" @close="modal.open = false" />
  </div>
</template>

<style scoped>
.win { height: 100%; display: flex; flex-direction: column; }
.dashboard { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 16px; padding: 20px 28px 12px; overflow-y: auto; width: 100%; max-width: 1120px; margin: 0 auto; }
.dashboard-header { display: flex; align-items: center; gap: 24px; flex: none; }
.brand { display: flex; align-items: center; gap: 12px; flex: none; }
.brand-logo { width: 38px; height: 38px; border-radius: 12px; box-shadow: 0 2px 6px #24382e0a; border: 1px solid #ffffffb0; }
.brand h1 { font-size: 21px; font-weight: 600; letter-spacing: -.03em; line-height: 1.4; }
.header-trail { flex: 1; min-width: 60px; max-width: 280px; height: 40px; display: flex; margin-left: auto; opacity: .85; }
.settings-btn { display: grid; place-items: center; flex: none; width: 34px; height: 34px; border: 1px solid #263e3010; border-radius: 50%; background: #ffffff70; color: var(--ink); cursor: pointer; transition: background .2s; }
.settings-btn:hover { background: #fff; }
.section-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.section-heading h2 { font-size: 12px; font-weight: 500; color: var(--muted); }
.transport-section { flex: none; }
.transport-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.transport-card { --accent: #3b9b74; display: flex; align-items: center; gap: 12px; text-align: left; padding: 12px 16px; min-width: 0; border: 1px solid #294d3610; border-radius: 15px; background: #ffffff70; color: var(--ink); cursor: pointer; transition: border-color .2s, background .2s, transform .2s; }
.transport-card.turtleor { --accent: #bb7496; }
.transport-card:hover:not(:disabled) { background: #ffffffd9; }
.transport-card:active:not(:disabled) { transform: scale(.99); }
.transport-card.selected { border-color: #a9d6bb; background: #e5f5eb; }
.transport-card.turtleor.selected { border-color: #e4bfd1; background: #f9eaf1; }
.transport-card:disabled { cursor: default; }
.transport-card:disabled:not(.selected) { opacity: .65; }
.transport-icon { width: 30px; height: 30px; display: grid; place-items: center; flex: none; border-radius: 9px; color: var(--accent); }
.transport-icon svg { width: 22px; height: 22px; }
.transport-copy { min-width: 0; }
.transport-copy strong { font-size: 14px; font-weight: 550; }
.selection-mark { width: 16px; height: 16px; border: 1px solid #293c301c; border-radius: 50%; margin-left: auto; flex: none; }
.selected .selection-mark { color: #fff; background: var(--accent); border-color: var(--accent); }
.selection-mark svg { width: 100%; height: 100%; display: block; }
.selected .selection-mark.searching { background: transparent; border: 1.5px solid #293c301c; border-top-color: var(--accent); animation: spin-anim .8s linear infinite; }
.overview { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); flex: none; overflow: hidden; }
.connection-card { padding: 24px 24px 20px; }
.connection-summary { display: flex; align-items: center; gap: 12px; min-height: 38px; }
.connection-summary h2 { display: flex; align-items: center; gap: 9px; font-size: 23px; font-weight: 550; letter-spacing: -.04em; line-height: 1.4; }
.connection-summary .status-dot { width: 7px; height: 7px; background: #bdc8c0; flex: none; }
.connection-summary .status-dot.on { background: #52b387; animation: none; }
.process-badge { color: var(--muted); font-size: 10px; }
.connect-btn { width: 100%; height: 40px; padding: 0 16px; display: flex; align-items: center; justify-content: center; gap: 8px; border-radius: 11px; font-size: 13px; font-weight: 500; }
.node-details { display: grid; grid-template-columns: 1.15fr 1fr 1fr; margin-top: 24px; }
.node-details > div { display: flex; flex-direction: column; gap: 7px; min-width: 0; padding-left: 16px; }
.node-details > div:first-child { padding-left: 0; }
.detail-label { color: var(--muted); font-size: 11px; }
.node-details strong { font-size: 15px; font-weight: 500; font-variant-numeric: tabular-nums; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.node-details small { font-weight: 400; font-size: 10px; color: var(--muted); }
.node-details .timeout { color: #c98037; }
.traffic-card { display: flex; flex-direction: column; margin: 24px 0 20px; padding: 0 24px; min-width: 0; border-left: 1px solid #283d3010; }
.traffic-values { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); flex: 1; align-items: end; gap: 12px; padding-top: 16px; }
.traffic-item { min-width: 0; }
.traffic-label { display: flex; align-items: center; gap: 5px; font-size: 11px; color: var(--muted); margin-bottom: 10px; }
.traffic-arrow { font-size: 15px; }
.upload { color: #51a580; }
.download { color: #c17c9c; }
.traffic-item strong { display: block; font-size: clamp(16px, 2.2vw, 24px); font-weight: 500; letter-spacing: -.04em; font-variant-numeric: tabular-nums; white-space: nowrap; }
.traffic-item small { font-size: 11px; font-weight: 400; color: var(--muted); letter-spacing: 0; margin-left: 4px; }
.connection-actions { grid-column: 1 / -1; display: grid; grid-template-columns: minmax(0, 1fr) 148px; align-items: center; gap: 24px; padding: 12px 24px; border-top: 1px solid #283d3010; background: #eff6f133; }
.endpoint-bar { display: flex; align-items: center; gap: 10px; min-width: 0; color: var(--muted); }
.protocol-tag { font-size: 10px; font-weight: 500; }
.copy-address { display: flex; align-items: center; gap: 10px; border: 0; background: none; color: var(--muted); font-family: 'Cascadia Mono', Consolas, monospace; font-size: 11px; cursor: pointer; padding: 3px; border-radius: 5px; user-select: text; }
.copy-address:hover { color: var(--ink); background: #e9f0eb; }
.copy-address svg { opacity: .5; }
.log-panel { flex: 1; min-height: 96px; display: flex; flex-direction: column; overflow: hidden; background: #ffffff80; box-shadow: none; }
.log-heading { display: flex; justify-content: space-between; align-items: center; padding: 12px 20px 8px; }
.text-button, .community-link { border: 0; background: none; font-size: 11px; color: var(--muted); cursor: pointer; padding: 3px 5px; border-radius: 5px; }
.text-button:hover:not(:disabled), .community-link:hover { color: var(--green-700); background: #e5f7eb; }
.text-button:disabled { opacity: .45; cursor: default; }
.log-list { flex: 1; min-height: 0; overflow-y: auto; padding: 4px 20px 10px; user-select: text; }
.log-line { display: grid; grid-template-columns: 56px minmax(0, 1fr); gap: 12px; padding: 5px 0; font: 11px/1.7 'Cascadia Mono', Consolas, 'Microsoft YaHei', monospace; }
.log-line time { color: #8b968f; font-size: 10px; font-variant-numeric: tabular-nums; }
.log-level { font-size: 10px; }
.log-message { overflow-wrap: anywhere; }
.log-line.warn .log-message, .log-line.warn .log-level { color: #b37b23; }
.log-line.error .log-message, .log-line.error .log-level { color: #d15c6b; }
.log-empty { display: grid; place-items: center; height: 100%; min-height: 36px; color: #98a19b; font-size: 11px; }
.dashboard-footer { display: flex; justify-content: flex-end; flex: none; margin-top: -8px; }
.community-link { font-size: 10px; color: #929c95; }
.dashboard-header, .transport-section, .overview { animation: rise .4s ease both; }
.transport-section { animation-delay: .04s; }
.overview { animation-delay: .08s; }
@keyframes rise { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
@media (max-height: 600px) {
  .dashboard { gap: 14px; padding-top: 16px; }
  .connection-card { padding: 20px; }
  .traffic-card { margin: 20px 0; padding: 0 20px; }
  .node-details { margin-top: 20px; }
  .connection-actions { padding: 10px 20px; }
}
@media (max-width: 800px) {
  .dashboard { padding-left: 18px; padding-right: 18px; }
  .connection-card { padding-left: 16px; padding-right: 16px; }
  .connection-summary h2 { font-size: 21px; }
  .connection-actions { padding-left: 16px; padding-right: 16px; }
  .traffic-card { padding: 0 16px; }
}
@media (max-width: 620px) {
  .dashboard { overflow-y: auto; }
  .header-trail { display: none; }
  .settings-btn { margin-left: auto; }
  .transport-grid, .overview { grid-template-columns: 1fr; }
  .traffic-card { margin: 0 20px 20px; padding: 20px 0 0; border-left: 0; border-top: 1px solid #283d3010; }
  .log-panel { flex: none; height: 180px; }
}
</style>
