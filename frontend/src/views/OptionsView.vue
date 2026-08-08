<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, onState } from '../bridge'
import { showToast } from '../toast'
import { candyBurst } from '../confetti'
import CandyToggle from '../components/CandyToggle.vue'
import CandyInput from '../components/CandyInput.vue'
import ModalDialog from '../components/ModalDialog.vue'
import TitleBar from '../components/TitleBar.vue'

const connected = ref(false)
const localPort = ref(7788)

const optSysProxy = ref(true)
const optAllowLan = ref(false)
const optSocks5 = ref(false)
const customPort = ref('')

const relayOn = ref(false)
const relayBusy = ref(false)
const relayPort = ref('8388')
const relayPassword = ref('')
const diceSpin = ref(false)

const modal = reactive({ open: false, title: '', lines: [] as string[], danger: false })

function showModal(title: string, lines: string[], danger = false) {
  modal.title = title
  modal.lines = lines
  modal.danger = danger
  modal.open = true
}

const displayPort = ref(7788)

async function save() {
  const err = await api.setOptions({
    useSocks5: optSocks5.value,
    allowLan: optAllowLan.value,
    sysProxy: optSysProxy.value,
    localPort: parsePort(customPort.value),
  } as any)
  if (err) {
    showToast('error', err, 2600)
    await loadOptions()
  }
}

function parsePort(s: string): number {
  const t = s.trim()
  if (!t) return 0
  const n = parseInt(t, 10)
  return Number.isNaN(n) ? -1 : n
}

async function loadOptions() {
  try {
    const o = await api.options()
    optSysProxy.value = o.sysProxy
    optAllowLan.value = o.allowLan
    optSocks5.value = o.useSocks5
    customPort.value = o.localPort > 0 ? String(o.localPort) : ''
    displayPort.value = o.localPort > 0 ? o.localPort : 7788
  } catch {}
}

function onSysProxyChange(v: boolean) {
  if (!v && !optSocks5.value) {
    showModal('提示', ['若关闭系统代理，你需要手动设置代理', `HTTP 127.0.0.1:${displayPort.value}`, '（端口被占用时会自动顺延）'])
  } else if (v) {
    optSocks5.value = false
  }
  save()
}

function onSocks5Change(v: boolean) {
  if (v) {
    showModal('提示', ['若使用 Socks5，你需要手动设置代理', `SOCKS5 127.0.0.1:${displayPort.value}`, '该选项和系统代理互斥', '（端口被占用时会自动顺延）'])
    optSysProxy.value = false
  }
  save()
}

function onPortCommit() {
  const n = parsePort(customPort.value)
  if (n === -1 || (n !== 0 && (n < 1024 || n > 65535))) {
    showToast('error', '端口需为 1024-65535，或留空使用默认 7788', 2600)
    loadOptions()
    return
  }
  displayPort.value = n > 0 ? n : 7788
  save()
}

async function onRelayChange(v: boolean) {
  if (relayBusy.value) return
  if (v && !connected.value) {
    showToast('warn', '建立连接后才可以开启 SS 中继')
    relayOn.value = false
    return
  }
  relayBusy.value = true
  try {
    if (v) {
      const port = parseInt(relayPort.value, 10)
      if (!port || port < 1 || port > 65535) {
        showToast('error', '中继端口无效')
        relayOn.value = false
        return
      }
      if (!relayPassword.value) {
        showToast('error', '请先生成或输入中继密码')
        relayOn.value = false
        return
      }
      const err = await api.startRelay(port, relayPassword.value)
      if (err) {
        showToast('error', err)
        relayOn.value = false
      } else {
        showModal('提示', [
          '请在局域网其他设备运行 Shadowsocks',
          '服务器: 本机内网 IP',
          `远程端口: ${port}`,
          `密码: ${relayPassword.value}`,
          '加密方式: AES-128-GCM',
        ])
      }
    } else {
      const err = await api.stopRelay()
      if (err) showToast('error', err)
    }
  } finally {
    relayBusy.value = false
  }
}

async function genPassword() {
  diceSpin.value = true
  window.setTimeout(() => (diceSpin.value = false), 500)
  try {
    relayPassword.value = await api.generatePassword()
  } catch {
    showToast('error', '密码生成失败')
  }
}

function closeWindow() {
  api.closeOptions()
}

function onGlobalPointerDown(ev: PointerEvent) {
  candyBurst(ev.clientX, ev.clientY)
}

onMounted(async () => {
  document.addEventListener('pointerdown', onGlobalPointerDown, true)
  onState((s) => {
    connected.value = s.state === 'connected'
    if (s.localPort) localPort.value = s.localPort
    if (s.relayActive !== relayOn.value && !relayBusy.value) relayOn.value = s.relayActive
  })
  await loadOptions()
  try {
    const s = await api.status()
    connected.value = s.state === 'connected'
    relayOn.value = s.relayActive
    if (s.localPort) localPort.value = s.localPort
  } catch {}
})
</script>

<template>
  <div class="win">
    <TitleBar title="选项" @close="closeWindow" />
    <div class="opt-body">
      <section class="card panel">
        <div class="card-title">代理选项</div>
        <CandyToggle v-model="optSysProxy" label="系统代理" :disabled="connected" @change="onSysProxyChange" />
        <CandyToggle v-model="optAllowLan" label="允许局域网连入" :disabled="connected" @change="save" />
        <CandyToggle v-model="optSocks5" label="使用 Socks5" :disabled="connected" @change="onSocks5Change" />
      </section>

      <section class="card panel">
        <div class="card-title">本地监听端口</div>
        <div class="port-row">
          <CandyInput
            v-model="customPort"
            placeholder="7788"
            :disabled="connected"
            @blur="onPortCommit"
            @keyup.enter="onPortCommit"
          />
        </div>
        <div class="hint">
          留空使用默认 7788，被占用时自动顺延
          <template v-if="connected"> · 当前生效 {{ localPort }}</template>
          <template v-else> · 下次连接使用 {{ displayPort }}</template>
        </div>
      </section>

      <section class="card panel">
        <div class="card-title">SS 局域网中继</div>
        <CandyToggle v-model="relayOn" label="开启中继" @change="onRelayChange" />
        <div class="relay-row">
          <CandyInput v-model="relayPort" placeholder="端口" :disabled="relayOn" />
          <CandyInput v-model="relayPassword" placeholder="密码" :disabled="relayOn" />
          <button class="dice-btn" :class="{ spin: diceSpin }" :disabled="relayOn" title="生成密码" @click="genPassword">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="3" width="18" height="18" rx="4" /><circle cx="8.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="8.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/></svg>
          </button>
        </div>
      </section>
    </div>

    <ModalDialog :open="modal.open" :title="modal.title" :lines="modal.lines" :danger="modal.danger" @close="modal.open = false" />
  </div>
</template>

<style scoped>
.win {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.opt-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 14px 14px;
}

.panel {
  padding: 14px 18px;
}

.panel:last-child {
  flex: 1;
  align-content: start;
}

.panel:nth-child(1) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.08s both;
}

.panel:nth-child(2) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.16s both;
}

.panel:nth-child(3) {
  animation: rise 0.5s cubic-bezier(0.22, 0.9, 0.36, 1) 0.24s both;
}

.panel:hover {
  box-shadow: var(--shadow-hover);
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

.port-row {
  margin-top: 10px;
  max-width: 180px;
}

.hint {
  margin-top: 8px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--green-600);
  line-height: 1.6;
}

.relay-row {
  display: grid;
  grid-template-columns: 90px 1fr 42px;
  gap: 8px;
  margin-top: 10px;
  align-items: center;
}

.dice-btn {
  height: 38px;
  border: none;
  border-radius: 12px;
  cursor: pointer;
  color: #fff;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, var(--pink-300), var(--pink-400));
  box-shadow: 0 4px 10px rgba(244, 114, 182, 0.35);
  transition: transform 0.15s cubic-bezier(0.34, 1.56, 0.64, 1), filter 0.2s ease, opacity 0.4s ease;
}

.dice-btn:hover:not(:disabled) {
  filter: brightness(1.08);
  transform: translateY(-2px);
}

.dice-btn:active:not(:disabled) {
  transform: scale(0.88);
}

.dice-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.dice-btn.spin svg {
  animation: dice-wiggle 0.5s ease;
}

@keyframes dice-wiggle {
  0% { transform: rotate(0); }
  25% { transform: rotate(-25deg) scale(1.15); }
  50% { transform: rotate(20deg) scale(1.15); }
  75% { transform: rotate(-10deg); }
  100% { transform: rotate(0); }
}
</style>
