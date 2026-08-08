<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { api, onLog, onOptions, onScan, onState } from '../bridge'
import { showToast } from '../toast'
import { candyBurst } from '../confetti'
import avatarUrl from '../assets/avatar.png'
import CandyButton from '../components/CandyButton.vue'
import ModalDialog from '../components/ModalDialog.vue'
import TitleBar from '../components/TitleBar.vue'
import TurtleRunner from '../components/TurtleRunner.vue'
import ConnectFlash from '../components/ConnectFlash.vue'
import NodeScan from '../components/NodeScan.vue'

interface TransportItem {
  key: string
  name: string
  desc: string
}

const transports: TransportItem[] = [
  { key: 'webtransport', name: 'WebTransport', desc: 'relay+wt 直连 (HTTP/3)' },
  { key: 'hysteria2', name: 'Hysteria2', desc: 'hy2 直连 端口跳跃' },
  { key: 'turtleor', name: 'Turtleor', desc: 'hy2 经 warpplus(M-H3)' },
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

const optSocks5 = ref(false)
const optAllowLan = ref(false)
const optCustomPort = ref(0)

const flash = ref<'' | 'connect' | 'disconnect'>('')
const flashKey = ref(0)
let flashTimer: number | undefined

function playFlash(type: 'connect' | 'disconnect') {
  if (flashTimer !== undefined) window.clearTimeout(flashTimer)
  flash.value = type
  flashKey.value++
  flashTimer = window.setTimeout(() => (flash.value = ''), 1650)
}

const modal = reactive({ open: false, title: '', lines: [] as string[], danger: false })

interface LogLine {
  id: number
  level: string
  msg: string
}
const logs = ref<LogLine[]>([])
let logId = 1

function pushLog(level: string, msg: string) {
  logs.value.unshift({ id: logId++, level, msg })
  if (logs.value.length > 60) logs.value.pop()
}

function showModal(title: string, lines: string[], danger = false) {
  modal.title = title
  modal.lines = lines
  modal.danger = danger
  modal.open = true
}

const SCAN_MIN_MS = 2600

async function pickTransport(t: TransportItem) {
  if (connected.value) {
    showToast('warn', '请先断开连接再切换传输方式')
    return
  }
  if (scanning.value) return
  activeTransport.value = t.key
  nodeLabel.value = '正在分配节点...'
  nodeName.value = ''
  hopPort.value = 0
  latency.value = -1
  scanning.value = true
  scanAttempt.value = 0
  scanHost.value = ''
  const scanStart = Date.now()
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
    if (res.reachable) {
      showToast('ok', res.latency > 0 ? `节点可用，延迟 ${res.latency}ms` : '节点可用')
    } else {
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
    const remain = SCAN_MIN_MS - (Date.now() - scanStart)
    if (remain > 0) await new Promise((r) => setTimeout(r, remain))
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
        playFlash('connect')
        showToast('ok', '已连接')
      }
    } catch (e: any) {
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
        playFlash('disconnect')
        showToast('ok', '已断开')
      }
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

function onGlobalPointerDown(ev: PointerEvent) {
  candyBurst(ev.clientX, ev.clientY)
}

onMounted(async () => {
  document.addEventListener('pointerdown', onGlobalPointerDown, true)
  pingTimer = window.setInterval(refreshLatency, 10000)
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
    connected.value = s.state === 'connected'
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
    const o = await api.options()
    optSocks5.value = o.useSocks5
    optAllowLan.value = o.allowLan
    optCustomPort.value = o.localPort
  } catch {}
  try {
    const s = await api.status()
    connected.value = s.state === 'connected'
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
  document.removeEventListener('pointerdown', onGlobalPointerDown, true)
  if (pingTimer !== undefined) window.clearInterval(pingTimer)
  if (flashTimer !== undefined) window.clearTimeout(flashTimer)
})
</script>

<template>
  <div class="win">
    <TitleBar title="神龟翻滚 SGFG-VUE" :version="version" show-min show-max @close="Window.Close()" />
    <div class="layout">
      <aside class="sidebar card">
      <div class="brand">
        <img class="brand-logo" :src="avatarUrl" alt="SGFG" />
        <div>
          <div class="brand-name">神龟翻滚</div>
          <div class="brand-sub">{{ version }}</div>
        </div>
      </div>

      <div class="menu-label">传输方式</div>
      <div class="menu">
        <div
          v-for="t in transports"
          :key="t.key"
          class="menu-item"
          :class="{ active: activeTransport === t.key, locked: connected || scanning }"
          @click="pickTransport(t)"
        >
          <div class="menu-item-name">{{ t.name }}</div>
          <div class="menu-item-desc">{{ t.desc }}</div>
        </div>
      </div>

      <div class="sidebar-footer">
        <button class="link-btn" @click="openTelegram">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M21.9 4.6c.2-1-.7-1.7-1.6-1.3L2.7 9.6c-1 .4-1 1.9.1 2.2l4.4 1.4 1.7 5.3c.3 1 1.5 1.3 2.2.5l2.5-2.6 4.5 3.3c.8.6 2 .2 2.2-.8l1.6-14.3ZM8.4 12.6l10.3-6.3c.2-.1.4.2.2.3l-8.5 7.7-.3 3-2-4.4c-.1-.1 0-.3.3-.3Z"/></svg>
          t.me/sgfangun
        </button>
      </div>
    </aside>

    <main class="main">
      <div class="topbar card">
        <div class="topbar-status">
          <span class="status-dot" :class="{ on: connected }"></span>
          <span class="topbar-text">{{ connected ? '已连接' : '未连接' }}</span>
        </div>
        <TurtleRunner />
        <div class="topbar-node">{{ nodeLabel }}</div>
        <button class="gear-btn" title="选项" @click="openOptions">
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z"/></svg>
        </button>
      </div>

      <div class="grid">
        <section class="card panel">
          <div class="card-title">节点信息</div>
          <div class="node-grid">
            <div class="node-item">
              <div class="node-key">节点</div>
              <div class="node-val">{{ nodeName || '—' }}</div>
            </div>
            <div class="node-item">
              <div class="node-key">端口</div>
              <div class="node-val">{{ hopPort || '—' }}</div>
            </div>
            <div class="node-item">
              <div class="node-key">延迟</div>
              <div class="node-val" :key="latencyTick">
                <span v-if="latency >= 0 && reachable" class="latency-pop">{{ latency }} ms</span>
                <span v-else-if="!reachable" class="latency-pop latency-timeout">超时</span>
                <span v-else>—</span>
              </div>
            </div>
          </div>
        </section>

        <section class="card panel connect-panel">
          <CandyButton
            class="connect-btn"
            :variant="connected ? 'danger' : 'primary'"
            :loading="connecting"
            @click="toggleConnect"
          >
            {{ connected ? '断开连接' : '建立连接' }}
          </CandyButton>
          <div class="connect-hint">
            本地 {{ optAllowLan ? '0.0.0.0' : '127.0.0.1' }}:{{ connected ? localPort : optCustomPort || 7788 }}
            · {{ optSocks5 ? 'socks5' : 'http' }}
          </div>
        </section>

        <section class="card panel log-panel">
          <div class="card-title">运行日志</div>
          <div class="log-list">
            <TransitionGroup name="list-anim">
              <div v-for="l in logs" :key="l.id" class="log-line" :class="l.level">{{ l.msg }}</div>
            </TransitionGroup>
            <div v-if="logs.length === 0" class="log-empty">暂无日志</div>
          </div>
        </section>
      </div>
    </main>
    </div>

    <Transition name="scan-fade">
      <NodeScan v-if="scanning" :host="scanHost" :attempt="scanAttempt" />
    </Transition>
    <ConnectFlash v-if="flash" :key="flashKey" :type="flash" />
    <ModalDialog :open="modal.open" :title="modal.title" :lines="modal.lines" :danger="modal.danger" @close="modal.open = false" />
  </div>
</template>

<style scoped>
.win {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.layout {
  flex: 1;
  min-height: 0;
  display: flex;
  padding: 12px 14px 14px;
  gap: 14px;
}

.sidebar {
  animation: rise 0.55s cubic-bezier(0.22, 0.9, 0.36, 1) both;
}

.topbar {
  animation: rise 0.55s cubic-bezier(0.22, 0.9, 0.36, 1) 0.08s both;
}

.grid .panel:nth-child(1) {
  animation: rise 0.55s cubic-bezier(0.22, 0.9, 0.36, 1) 0.14s both;
}

.grid .panel:nth-child(2) {
  animation: rise 0.55s cubic-bezier(0.22, 0.9, 0.36, 1) 0.2s both;
}

.grid .panel:nth-child(3) {
  animation: rise 0.55s cubic-bezier(0.22, 0.9, 0.36, 1) 0.26s both;
}

.menu-item:nth-child(1) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.18s both;
}

.menu-item:nth-child(2) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.26s both;
}

.menu-item:nth-child(3) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.34s both;
}

@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(20px) scale(0.97);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.scan-fade-enter-active {
  transition: all 0.35s cubic-bezier(0.34, 1.4, 0.64, 1);
}

.scan-fade-leave-active {
  transition: all 0.3s ease-in;
}

.scan-fade-enter-from,
.scan-fade-leave-to {
  opacity: 0;
}

.scan-fade-enter-from :deep(.scan-stage),
.scan-fade-leave-to :deep(.scan-stage) {
  transform: scale(0.85);
}

.scan-fade-enter-active :deep(.scan-stage),
.scan-fade-leave-active :deep(.scan-stage) {
  transition: transform 0.35s cubic-bezier(0.34, 1.4, 0.64, 1);
}

.sidebar {
  width: 230px;
  flex: none;
  display: flex;
  flex-direction: column;
  padding: 18px 14px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 6px 16px;
}

.brand-logo {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: block;
  object-fit: cover;
  box-shadow: 0 6px 14px rgba(16, 185, 129, 0.35);
}

.brand-name {
  font-size: 17px;
  font-weight: 800;
  color: var(--green-800);
}

.brand-sub {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  color: var(--green-500);
}

.menu-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--green-600);
  padding: 6px 8px;
}

.menu {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.menu-item {
  padding: 12px 14px;
  border-radius: 14px;
  cursor: pointer;
  border: 2px solid transparent;
  background: rgba(255, 255, 255, 0.55);
  transition: all 0.22s cubic-bezier(0.34, 1.4, 0.64, 1);
}

.menu-item:hover:not(.locked) {
  transform: translateX(4px);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.15);
}

.menu-item:active:not(.locked) {
  transform: translateX(4px) scale(0.96);
}

.menu-item.active {
  border-color: var(--green-400);
  background: linear-gradient(135deg, rgba(167, 243, 208, 0.7), rgba(209, 250, 229, 0.9));
  box-shadow: 0 6px 16px rgba(16, 185, 129, 0.25);
}

.menu-item.locked {
  opacity: 0.55;
  cursor: not-allowed;
}

.menu-item-name {
  font-size: 14.5px;
  font-weight: 800;
  color: var(--green-800);
}

.menu-item-desc {
  font-size: 11.5px;
  color: var(--green-600);
  margin-top: 2px;
}

.sidebar-footer {
  margin-top: auto;
  padding-top: 12px;
}

.link-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 9px 12px;
  border: none;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.6);
  color: var(--green-700);
  font-family: inherit;
  font-size: 12.5px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s ease;
}

.link-btn:hover {
  background: #fff;
  transform: translateY(-2px);
  box-shadow: 0 6px 14px rgba(16, 185, 129, 0.2);
}

.link-btn:active {
  transform: scale(0.95);
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.topbar {
  display: flex;
  align-items: center;
  padding: 12px 20px;
  gap: 12px;
}

.topbar-status {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

.topbar-text {
  font-size: 14px;
  font-weight: 800;
  color: var(--green-800);
}

.topbar-node {
  font-size: 13px;
  font-weight: 700;
  color: var(--green-600);
  white-space: nowrap;
}

.gear-btn {
  flex: none;
  width: 34px;
  height: 34px;
  border: none;
  border-radius: 12px;
  display: grid;
  place-items: center;
  cursor: pointer;
  color: var(--green-700);
  background: rgba(255, 255, 255, 0.65);
  transition: all 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.gear-btn:hover {
  background: #fff;
  transform: translateY(-2px);
  box-shadow: 0 6px 14px rgba(16, 185, 129, 0.25);
}

.gear-btn:hover svg {
  transform: rotate(40deg);
}

.gear-btn svg {
  transition: transform 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.gear-btn:active {
  transform: scale(0.88);
}

.gear-btn.close-win:hover {
  color: var(--red-500);
  box-shadow: 0 6px 14px rgba(239, 68, 68, 0.25);
}

.grid {
  flex: 1;
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  grid-template-rows: auto 1fr;
  gap: 14px;
  min-height: 0;
  overflow-y: auto;
  padding: 2px;
}

.panel {
  padding: 14px 18px;
}

.panel:hover {
  box-shadow: var(--shadow-hover);
}

.node-grid {
  display: flex;
  gap: 18px;
  margin-top: 10px;
}

.node-item {
  flex: 1;
  min-width: 0;
}

.node-key {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--green-600);
  margin-bottom: 4px;
}

.node-val {
  font-size: 15px;
  font-weight: 800;
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.latency-pop {
  display: inline-block;
  animation: latency-in 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.latency-timeout {
  color: var(--red-500);
}

@keyframes latency-in {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.8);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.connect-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 14px 18px;
}

.connect-btn {
  width: 100%;
  padding: 15px 0;
  font-size: 17px;
  letter-spacing: 0.2em;
}

.connect-hint {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--green-600);
}

.log-panel {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.log-list {
  margin-top: 8px;
  flex: 1;
  min-height: 70px;
  overflow-y: auto;
  font-size: 12.5px;
  line-height: 1.7;
  font-family: 'Cascadia Mono', 'JetBrains Mono', Consolas, 'Microsoft YaHei', monospace;
  user-select: text;
}

.log-line {
  padding: 3px 6px;
  border-radius: 6px;
  color: var(--green-800);
}

.log-line.warn {
  color: #b45309;
}

.log-line.error {
  color: var(--red-500);
}

.log-empty {
  color: #9ca3af;
  padding: 8px 6px;
}
</style>
