<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { ConnectOptions, RelayOptions } from '../../bindings/sgfg-ng/internal/proxy/models'
import { createSerialSaver } from '../serialSaver'
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
const optProcessProxy = ref(false)
const processNames = ref('')
const ready = ref(false)
const pendingSaves = ref(0)
const saving = computed(() => pendingSaves.value > 0)
const savedOptions = ref<ConnectOptions | null>(null)
const saveSnapshot = createSerialSaver<ConnectOptions>(api.setOptions)
const unsaved = computed(() => ready.value && JSON.stringify(optionSnapshot()) !== JSON.stringify(savedOptions.value))

const relayOn = ref(false)
const relayBusy = ref(false)
const relayPort = ref('8388')
const relayPassword = ref('')
const diceSpin = ref(false)
const relayPending = ref(0)
const relaySaving = computed(() => relayPending.value > 0)
const savedRelay = ref<RelayOptions | null>(null)
const saveRelaySnapshot = createSerialSaver<RelayOptions>(api.setRelayOptions)
const relayUnsaved = computed(() => ready.value && JSON.stringify(relaySnapshot()) !== JSON.stringify(savedRelay.value))

function relaySnapshot(): RelayOptions {
  return { port: parsePort(relayPort.value), password: relayPassword.value }
}

async function saveRelay(): Promise<boolean> {
  if (!ready.value) return false
  const snapshot = relaySnapshot()
  relayPending.value++
  try {
    savedRelay.value = await saveRelaySnapshot(snapshot)
    return true
  } catch (err) {
    showToast('error', String(err), 3500)
    return false
  } finally { relayPending.value-- }
}

function discardRelay() {
  if (!savedRelay.value || relaySaving.value) return
  relayPort.value = String(savedRelay.value.port)
  relayPassword.value = savedRelay.value.password
}

const modal = reactive({ open: false, title: '', lines: [] as string[], danger: false })

function showModal(title: string, lines: string[], danger = false) {
  modal.title = title
  modal.lines = lines
  modal.danger = danger
  modal.open = true
}

const displayPort = ref(7788)

function optionSnapshot(): ConnectOptions {
  return {
    useSocks5: optSocks5.value,
    allowLan: optAllowLan.value,
    sysProxy: optSysProxy.value,
    localPort: parsePort(customPort.value),
    processProxy: optProcessProxy.value,
    processNames: [...new Set(processNames.value.split(/[\n,;，；]+/).map(s => s.trim().toLowerCase()).filter(Boolean))],
  }
}

async function save() {
  if (!ready.value) return
  const snapshot = optionSnapshot()
  pendingSaves.value++
  try {
    // Only update the saved baseline. Never write a response into the draft.
    savedOptions.value = await saveSnapshot(snapshot)
  } catch (err) {
    showToast('error', String(err), 3500)
  } finally {
    pendingSaves.value--
  }
}

function parsePort(s: string): number {
  const t = s.trim()
  if (!t) return 0
  return /^\d+$/.test(t) ? Number(t) : -1
}

async function loadOptions() {
  try {
    const [o, r] = await Promise.all([api.options(), api.relayOptions()])
    optSysProxy.value = o.sysProxy
    optAllowLan.value = o.allowLan
    optSocks5.value = o.useSocks5
    optProcessProxy.value = o.processProxy
    processNames.value = (o.processNames ?? []).join('\n')
    customPort.value = o.localPort > 0 ? String(o.localPort) : ''
    displayPort.value = o.localPort > 0 ? o.localPort : 7788
    savedOptions.value = optionSnapshot()
    relayPort.value = String(r.port)
    relayPassword.value = r.password
    savedRelay.value = relaySnapshot()
    ready.value = true
  } catch (err) { showToast('error', `读取选项失败，请重新打开窗口：${err}`) }
}

function discardDraft() {
  const o = savedOptions.value
  if (!o || saving.value) return
  optSysProxy.value = o.sysProxy
  optAllowLan.value = o.allowLan
  optSocks5.value = o.useSocks5
  optProcessProxy.value = o.processProxy
  processNames.value = o.processNames.join('\n')
  customPort.value = o.localPort ? String(o.localPort) : ''
}

function onSysProxyChange(v: boolean) {
  if (!v && !optSocks5.value) {
    showModal('提示', ['若关闭系统代理，你需要手动设置代理', `HTTP 127.0.0.1:${displayPort.value}`, '（端口被占用时会自动顺延）'])
  } else if (v) {
    optSocks5.value = false
  }
  save()
}

function onProcessProxyChange(v: boolean) {
  if (v) {
    optSocks5.value = true
    optSysProxy.value = false
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
      const port = parsePort(relayPort.value)
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
      if (!(await saveRelay())) { relayOn.value = false; return }
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
    await saveRelay()
  } catch {
    showToast('error', '密码生成失败')
  }
}

function closeWindow() {
  if (saving.value || unsaved.value || relaySaving.value || relayUnsaved.value || relayBusy.value) {
    showToast('warn', '请先保存或撤销修改', 3500)
    return
  }
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
        <CandyToggle v-model="optSysProxy" label="系统代理" :disabled="!ready || saving || connected || optProcessProxy" @change="onSysProxyChange" />
        <CandyToggle v-model="optAllowLan" label="允许局域网连入" :disabled="!ready || saving || connected" @change="save" />
        <CandyToggle v-model="optSocks5" label="使用 Socks5" :disabled="!ready || saving || connected || optProcessProxy" @change="onSocks5Change" />
      </section>

      <section class="card panel">
        <div class="card-title">进程代理</div>
        <label class="hint" for="process-names">进程文件名（每行一个，也可用逗号分隔）</label>
        <textarea id="process-names" v-model="processNames" class="process-input"
          placeholder="chrome.exe&#10;firefox.exe" :disabled="!ready || connected" spellcheck="false" />
        <button class="save-processes" :disabled="!ready || saving || connected" @click="save">{{ saving ? '保存中…' : '保存设置' }}</button>
        <button v-if="unsaved && !saving" class="save-processes" @click="discardDraft">撤销修改</button>
        <div class="hint" role="status">
          {{ !ready ? '正在加载…' : saving ? '正在保存…' : unsaved ? '有未保存的修改' : connected ? '设置已生效' : '已保存，连接后生效' }}
        </div>
        <CandyToggle v-model="optProcessProxy" label="仅代理以上进程" :disabled="!ready || saving || connected" @change="onProcessProxyChange" />
        <div class="hint">
          仅代理列表中的应用，其他应用直连。进程名不区分大小写，新建连接生效。
          连接时自动加载驱动，断开后卸载；与系统代理互斥。设置保存在本机，下次启动自动恢复。
        </div>
        <details class="hint">
          <summary>功能说明</summary>
          <p>进程代理需要管理员权限。同名进程均适用，其他名称的子进程需单独添加。</p>
          <p>DNS（UDP/TCP 53）通过节点使用 1.1.1.1（TCP）；系统 DNS 服务与加密 DNS 不改写。</p>
          <p>WebTransport 仅支持 TCP，其他非本地 UDP 不转发；Hysteria2 与 Turtleor 支持 UDP。</p>
          <p>随附驱动为评估版，存在连接数量限制。断开或退出后不再限制应用直连。</p>
        </details>
      </section>

      <section class="card panel">
        <div class="card-title">本地监听端口</div>
        <div class="port-row">
          <CandyInput
            v-model="customPort"
            placeholder="7788"
            :disabled="!ready || saving || connected"
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
        <CandyToggle v-model="relayOn" label="开启中继" :disabled="!ready || relaySaving || relayBusy" @change="onRelayChange" />
        <div class="relay-row">
          <CandyInput v-model="relayPort" placeholder="端口" :disabled="!ready || relayOn || relayBusy" @blur="saveRelay" @keyup.enter="saveRelay" />
          <CandyInput v-model="relayPassword" placeholder="密码" :disabled="!ready || relayOn || relayBusy" @blur="saveRelay" @keyup.enter="saveRelay" />
          <button class="dice-btn" :class="{ spin: diceSpin }" :disabled="!ready || relayOn || relayBusy || relaySaving || diceSpin" title="生成密码" @click="genPassword">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="3" width="18" height="18" rx="4" /><circle cx="8.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="8.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/></svg>
          </button>
        </div>
        <button class="save-processes" :disabled="!ready || relayOn || relayBusy || relaySaving" @click="saveRelay">{{ relaySaving ? '保存中…' : '保存中继设置' }}</button>
        <button v-if="relayUnsaved && !relaySaving && !relayOn" class="save-processes" @click="discardRelay">撤销修改</button>
        <div class="hint" role="status">{{ relaySaving ? '正在保存…' : relayUnsaved ? '有未保存的修改' : ready ? '已保存到本机（密码加密）' : '正在加载…' }} · 中继需手动开启</div>
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

.process-input {
  display: block;
  width: 100%;
  min-height: 76px;
  box-sizing: border-box;
  margin: 8px 0;
  padding: 10px 12px;
  resize: vertical;
  border: 1px solid var(--green-300);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.75);
  color: var(--green-700);
  font: inherit;
}

.process-input:disabled { opacity: 0.55; }

.save-processes {
  padding: 6px 12px;
  margin-bottom: 8px;
  border: 1px solid var(--green-300);
  border-radius: 10px;
  background: var(--green-100);
  color: var(--green-700);
  cursor: pointer;
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
