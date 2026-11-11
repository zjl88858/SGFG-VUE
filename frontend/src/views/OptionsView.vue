<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { ConnectOptions, RelayOptions } from '../../bindings/sgfg-ng/internal/proxy/models'
import { createSerialSaver } from '../serialSaver'
import { api, onState } from '../bridge'
import { showToast } from '../toast'
import CandyToggle from '../components/CandyToggle.vue'
import CandyInput from '../components/CandyInput.vue'
import ModalDialog from '../components/ModalDialog.vue'
import TitleBar from '../components/TitleBar.vue'

const connected = ref(false)
const section = ref('general')
const sections = [
  { key: 'general', label: '常规' },
  { key: 'process', label: '进程代理' },
  { key: 'relay', label: '局域网中继' },
]
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
  displayPort.value = o.localPort || 7788
}

function onSysProxyChange(v: boolean) {
  if (v) optSocks5.value = false
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

onMounted(async () => {
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
    <TitleBar title="SGFG" subtitle="偏好设置" @close="closeWindow" />
    <div class="opt-body">
      <header class="options-header"><h1>偏好设置</h1></header>
      <nav class="settings-nav" aria-label="设置分区">
        <button v-for="item in sections" :key="item.key" :class="{ active: section === item.key }" :aria-pressed="section === item.key" @click="section = item.key">{{ item.label }}<i v-if="(item.key === 'relay' && relayUnsaved) || (item.key !== 'relay' && unsaved)" class="draft-dot" aria-label="有未保存的修改"></i></button>
      </nav>

      <section v-show="section === 'general'" class="card panel" aria-label="常规代理">
        <CandyToggle v-model="optSysProxy" label="系统代理" :disabled="!ready || saving || connected || optProcessProxy" @change="onSysProxyChange" />
        <CandyToggle v-model="optAllowLan" label="允许局域网连入" :disabled="!ready || saving || connected" @change="save" />
        <CandyToggle v-model="optSocks5" label="使用 SOCKS5" :disabled="!ready || saving || connected || optProcessProxy" @change="onSocks5Change" />
        <div class="port-setting"><label for="local-port"><strong>监听端口</strong></label><CandyInput id="local-port" v-model="customPort" placeholder="7788" title="留空使用 7788，占用时自动顺延" inputmode="numeric" :disabled="!ready || saving || connected" @blur="onPortCommit" @keyup.enter="onPortCommit" /></div>
        <div class="address-preview"><span>{{ optSocks5 ? 'SOCKS5' : 'HTTP' }}</span><code>{{ optAllowLan ? '0.0.0.0' : '127.0.0.1' }}:{{ connected ? localPort : displayPort }}</code></div>
        <div v-if="unsaved && !saving" class="action-row"><button class="save-processes" :disabled="!ready || connected" @click="onPortCommit">保存设置</button><button class="save-processes secondary" @click="discardDraft">撤销修改</button></div>
      </section>

      <section v-show="section === 'process'" class="card panel" aria-label="进程代理">
        <CandyToggle v-model="optProcessProxy" label="启用进程代理" :disabled="!ready || saving || connected" @change="onProcessProxyChange" />
        <label class="field-label" for="process-names">进程文件名</label>
        <textarea id="process-names" v-model="processNames" class="process-input"
          placeholder="chrome.exe&#10;firefox.exe" :disabled="!ready || connected" spellcheck="false" />
        <div class="action-row"><button class="save-processes" :disabled="!ready || saving || connected" @click="save">{{ saving ? '保存中…' : '保存' }}</button><button v-if="unsaved && !saving" class="save-processes secondary" @click="discardDraft">撤销</button></div>
        <details class="hint process-details">
          <summary>进程代理说明</summary>
          <p>列表中的应用走代理，其他应用直连；进程名不区分大小写。断开后自动卸载驱动。</p>
          <p>仅连接进程代理时为驱动助手请求管理员授权；主界面无需提权。同名进程均适用，其他名称的子进程需单独添加。</p>
          <p>DNS（UDP/TCP 53）通过节点使用 1.1.1.1（TCP）；系统 DNS 服务与加密 DNS 不改写。</p>
          <p>omhy-direct 与 omhy-turtleor 均使用 COPA 和 Blink，支持 TCP 与 UDP 转发。</p>
          <p>随附驱动为评估版，存在连接数量限制。断开或退出后不再限制应用直连。</p>
        </details>
      </section>

      <section v-show="section === 'relay'" class="card panel" aria-label="局域网中继">
        <CandyToggle v-model="relayOn" label="开启 SS 中继" :title="connected ? '' : '连接后可开启'" :disabled="!ready || !connected || relaySaving || relayBusy" @change="onRelayChange" />
        <div class="relay-row">
          <label class="field-label">端口<CandyInput v-model="relayPort" placeholder="8388" inputmode="numeric" :disabled="!ready || relayOn || relayBusy" @blur="saveRelay" @keyup.enter="saveRelay" /></label>
          <label class="field-label">连接密码<CandyInput v-model="relayPassword" placeholder="输入或生成密码" :disabled="!ready || relayOn || relayBusy" @blur="saveRelay" @keyup.enter="saveRelay" /></label>
          <button class="dice-btn" :class="{ rolling: diceSpin }" :disabled="!ready || relayOn || relayBusy || relaySaving || diceSpin" title="生成密码" aria-label="生成密码" @click="genPassword">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="3" width="18" height="18" rx="4" /><circle cx="8.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="15.5" cy="8.5" r="1.3" fill="currentColor" stroke="none"/><circle cx="8.5" cy="15.5" r="1.3" fill="currentColor" stroke="none"/></svg>
          </button>
        </div>
        <div class="action-row"><button class="save-processes" :disabled="!ready || relayOn || relayBusy || relaySaving" @click="saveRelay">{{ relaySaving ? '保存中…' : '保存' }}</button><button v-if="relayUnsaved && !relaySaving && !relayOn" class="save-processes secondary" @click="discardRelay">撤销</button></div>
        <div class="relay-info"><span>加密方式</span><span>AES-128-GCM</span></div>
      </section>
      <footer class="settings-footer"><span role="status">{{ !ready ? '加载中…' : saving || relaySaving ? '保存中…' : unsaved || relayUnsaved ? '未保存' : '已保存' }}</span><span v-if="connected">断开后可修改</span></footer>
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
  gap: 16px;
  min-height: 0;
  overflow-y: auto;
  padding: 22px 22px 14px;
}

.panel {
  padding: 18px 20px;
  flex: none;
}

.options-header { display: flex; align-items: center; justify-content: space-between; flex: none; }
.options-header h1 { font-size: 21px; font-weight: 550; letter-spacing: -.03em; }
.settings-nav { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 3px; padding: 4px; border: 1px solid #293c3008; border-radius: 11px; background: #e6ede7; flex: none; }
.settings-nav button { display: flex; align-items: center; justify-content: center; gap: 7px; padding: 10px 5px; background: transparent; border: 0; border-radius: 10px; color: var(--muted); font-size: 12px; cursor: pointer; transition: background .2s, box-shadow .2s; }
.settings-nav button.active { color: var(--ink); font-weight: 500; background: #fff; box-shadow: 0 1px 4px #293c3012; }
.settings-nav button:hover:not(.active) { background: #ffffff55; }
.draft-dot { width: 4px; height: 4px; border-radius: 50%; background: #d39c42; }
.panel :deep(.toggle-row) { min-height: 49px; border-radius: 0; padding: 12px 0; }
.panel :deep(.toggle-row + .toggle-row) { border-top: 1px solid #293c300a; }
.port-setting { display: grid; grid-template-columns: 1fr 100px; gap: 16px; align-items: center; border-top: 1px solid #e4efe7; margin-top: 8px; padding-top: 16px; }
.port-setting label { display: flex; flex-direction: column; gap: 5px; }
.port-setting strong { font-size: 13px; font-weight: 400; }
.address-preview { display: flex; align-items: center; gap: 10px; padding: 4px 0; margin-top: 16px; color: var(--muted); }
.address-preview > span { font-size: 10px; }
.address-preview code { font-size: 12px; user-select: text; }
.field-label { display: block; font-size: 12px; font-weight: 400; margin-top: 14px; color: var(--muted); }
.field-label :deep(input) { margin-top: 8px; }
.action-row { display: flex; align-items: center; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
.process-details { padding-top: 12px; border-top: 1px solid #e4efe7; }
.process-details summary { cursor: pointer; font-size: 11px; }
.process-details p { margin-top: 8px; font-weight: 400; }
.settings-footer { margin-top: auto; padding: 0 2px; display: flex; justify-content: space-between; gap: 10px; color: var(--muted); font-size: 10px; flex: none; }
.relay-info { display: flex; align-items: center; justify-content: space-between; padding: 16px 0 0; border-top: 1px solid #293c300a; margin-top: 22px; font-size: 11px; color: var(--muted); }

.process-input {
  display: block;
  width: 100%;
  min-height: 114px;
  box-sizing: border-box;
  margin: 8px 0;
  padding: 10px 12px;
  resize: vertical;
  border: 1px solid #cce5d5;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.75);
  color: var(--ink);
  font: 12px/1.8 'Cascadia Mono', Consolas, 'Microsoft YaHei', monospace;
  outline: none;
}

.process-input:disabled { opacity: 0.55; }
.process-input:focus { border-color: var(--green-400); box-shadow: 0 0 0 3px #34d39920; }

.save-processes {
  padding: 8px 12px;
  border: 1px solid #c6e7d4;
  border-radius: 10px;
  background: #e3f6eb;
  color: var(--green-700);
  cursor: pointer;
  font-size: 11px;
  font-weight: 500;
}
.save-processes:hover:not(:disabled) { background: #d1f1df; }
.save-processes.secondary { background: transparent; border-color: transparent; color: var(--muted); }
.save-processes:disabled { opacity: .45; cursor: not-allowed; }

.hint {
  margin-top: 14px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--muted);
  line-height: 1.6;
}

.relay-row {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr) 38px;
  gap: 8px;
  margin-top: 0;
  align-items: end;
}

.dice-btn {
  height: 38px;
  border: none;
  border-radius: 12px;
  cursor: pointer;
  color: #fff;
  display: grid;
  place-items: center;
  background: #f3e1ea;
  color: #b27292;
  box-shadow: none;
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

.dice-btn.rolling svg {
  animation: dice-wiggle 0.5s ease;
}

@media (max-width: 460px) {
  .opt-body { padding: 16px; }
  .panel { padding: 16px; }
  .settings-nav button { gap: 4px; font-size: 11px; }
  .relay-row { grid-template-columns: 65px minmax(0, 1fr) 36px; }
}

@media (max-height: 560px) {
  .opt-body { padding: 14px 16px; gap: 10px; }
  .options-header h1 { font-size: 19px; }
  .panel { padding: 14px 16px; }
  .panel :deep(.toggle-row) { padding-top: 9px; padding-bottom: 9px; }
  .port-setting { padding-top: 12px; }
  .address-preview { margin-top: 12px; }
  .process-input { min-height: 94px; }
  .relay-info { margin-top: 16px; }
}

@keyframes dice-wiggle {
  0% { transform: rotate(0); }
  25% { transform: rotate(-25deg) scale(1.15); }
  50% { transform: rotate(20deg) scale(1.15); }
  75% { transform: rotate(-10deg); }
  100% { transform: rotate(0); }
}
</style>
