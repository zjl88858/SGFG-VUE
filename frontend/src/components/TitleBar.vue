<script setup lang="ts">
import { Window } from '@wailsio/runtime'

defineProps<{
  title: string
  version?: string
  avatar?: string
  showMin?: boolean
  showMax?: boolean
}>()

const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <div class="titlebar drag-region" @dblclick="showMax ? Window.ToggleMaximise() : undefined">
    <div class="tb-left">
      <img v-if="avatar" :src="avatar" class="tb-avatar" alt="" />
      <span class="tb-title">{{ title }}</span>
      <span v-if="version" class="tb-ver">{{ version }}</span>
    </div>
    <div class="tb-actions no-drag">
      <button v-if="showMin" class="tb-btn min" title="最小化" @click="Window.Minimise()">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M5 12h14"/></svg>
      </button>
      <button v-if="showMax" class="tb-btn max" title="最大化" @click="Window.ToggleMaximise()">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="5" width="14" height="14" rx="2.5"/></svg>
      </button>
      <button class="tb-btn close" title="关闭" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  flex: none;
  height: 46px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px 0 14px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.88), rgba(209, 250, 229, 0.78));
  border-bottom: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 2px 12px rgba(16, 185, 129, 0.1);
  backdrop-filter: blur(10px);
  user-select: none;
  animation: tb-drop 0.45s cubic-bezier(0.22, 0.9, 0.36, 1) both;
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
  gap: 9px;
  min-width: 0;
}

.tb-avatar {
  width: 26px;
  height: 26px;
  border-radius: 9px;
  object-fit: cover;
  box-shadow: 0 3px 8px rgba(16, 185, 129, 0.3);
}

.tb-title {
  font-size: 13.5px;
  font-weight: 800;
  letter-spacing: 0.04em;
  color: var(--green-800);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tb-ver {
  flex: none;
  font-size: 10.5px;
  font-weight: 700;
  padding: 2px 9px;
  border-radius: 999px;
  color: var(--green-700);
  background: linear-gradient(135deg, var(--green-200), var(--green-100));
  box-shadow: 0 2px 6px rgba(16, 185, 129, 0.2);
}

.tb-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tb-btn {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 10px;
  display: grid;
  place-items: center;
  cursor: pointer;
  color: var(--green-700);
  background: transparent;
  transition: all 0.18s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.tb-btn.min:hover {
  background: var(--green-100);
  color: var(--green-600);
  box-shadow: 0 3px 8px rgba(16, 185, 129, 0.2);
}

.tb-btn.max:hover {
  background: #dbeafe;
  color: #2563eb;
  box-shadow: 0 3px 8px rgba(59, 130, 246, 0.2);
}

.tb-btn.close:hover {
  background: linear-gradient(135deg, var(--red-400), var(--red-500));
  color: #fff;
  box-shadow: 0 3px 10px rgba(239, 68, 68, 0.4);
}

.tb-btn.close:hover svg {
  transform: rotate(90deg);
}

.tb-btn.close svg {
  transition: transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.tb-btn:active {
  transform: scale(0.85);
}
</style>
