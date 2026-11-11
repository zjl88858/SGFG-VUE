<script setup lang="ts">
import { Window } from '@wailsio/runtime'

defineProps<{
  title: string
  subtitle?: string
  version?: string
  showMin?: boolean
  showMax?: boolean
}>()

const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <div class="titlebar drag-region" @dblclick="showMax ? Window.ToggleMaximise() : undefined">
    <div class="tb-left">
      <div class="tb-brand">
        <span class="tb-title">{{ title }}</span>
        <span v-if="version" class="tb-ver" :aria-label="`版本 ${version}`">{{ version }}</span>
      </div>
      <span v-if="subtitle" class="tb-subtitle">{{ subtitle }}</span>
    </div>
    <div class="tb-actions no-drag" @dblclick.stop>
      <button v-if="showMin" class="tb-btn min" title="最小化" aria-label="最小化" @click="Window.Minimise()">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M5 12h14"/></svg>
      </button>
      <button v-if="showMax" class="tb-btn max" title="最大化" aria-label="最大化或还原" @click="Window.ToggleMaximise()">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="5" width="14" height="14" rx="2.5"/></svg>
      </button>
      <button class="tb-btn close" title="关闭" aria-label="关闭" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  position: relative;
  flex: none;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px 0 20px;
  background: linear-gradient(105deg, #def1e5e6, #f8fcf9ed 42%, #f9eef4d9);
  border-bottom: 1px solid #ffffffb3;
  box-shadow: inset 0 1px 0 #ffffffed, 0 1px 0 #50745e0a;
  backdrop-filter: blur(16px);
  user-select: none;
  animation: tb-drop 0.45s cubic-bezier(0.22, 0.9, 0.36, 1) both;
}

.titlebar::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, #8abf9e60, #b9d6c226 42%, #d7abc34d);
  pointer-events: none;
}

@keyframes tb-drop {
  from {
    opacity: 0;
    transform: translateY(-100%);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.tb-left {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.tb-brand {
  display: inline-flex;
  align-items: center;
  gap: 11px;
  flex: none;
}

.tb-title {
  flex: none;
  font-size: 14px;
  font-weight: 750;
  letter-spacing: 0.12em;
  color: #3e7155;
  line-height: 24px;
  text-shadow: 0 1px 0 #ffffffa6;
}

.tb-subtitle {
  font-size: 11px;
  font-weight: 450;
  letter-spacing: 0.01em;
  color: #7c8a81;
  line-height: 20px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tb-ver {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 46px;
  height: 22px;
  padding: 0 10px;
  font-size: 10px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.04em;
  line-height: 1;
  border-radius: 999px;
  color: #a26183;
  background: linear-gradient(160deg, #fff6fb, #f2dce8);
  border: 1px solid #d8acc352;
  box-shadow: inset 0 1px 0 #ffffffed, 0 2px 5px #ac729210;
}

.tb-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  margin-left: 16px;
}

.tb-btn {
  width: 28px;
  height: 26px;
  border: 1px solid #ffffff80;
  border-radius: 8px;
  display: grid;
  place-items: center;
  cursor: pointer;
  color: #688473;
  background: #ffffff45;
  transition: background 0.18s ease, color 0.18s ease, transform 0.12s ease;
}

.tb-btn.min:hover,
.tb-btn.max:hover {
  background: #dcefe3;
  color: #3e7859;
}

.tb-btn.close {
  color: #a57b91;
  background: #f8e7ef80;
}

.tb-btn.close:hover {
  background: #f1d6e3;
  color: #a45579;
}

.tb-btn:active {
  transform: scale(0.94);
}
</style>
