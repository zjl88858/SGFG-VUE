<script setup lang="ts">
import { toasts } from '../toast'
</script>

<template>
  <div class="toast-host" role="status" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="t.type">
        <span class="toast-icon">
          <svg v-if="t.type === 'ok'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5" /></svg>
          <svg v-else-if="t.type === 'error'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          <svg v-else-if="t.type === 'warn'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M12 5v9M12 18.5v.5" /></svg>
          <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 7.5V8" /></svg>
        </span>
        <span class="toast-text">{{ t.text }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 50px;
  right: 16px;
  z-index: 999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}

.toast {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 160px;
  max-width: min(340px, calc(100vw - 32px));
  padding: 10px 14px;
  border-radius: 14px;
  font-size: 12px;
  font-weight: 400;
  color: var(--ink);
  background: #ffffffed;
  border: 1px solid #293c300d;
  backdrop-filter: blur(16px);
  box-shadow: 0 4px 20px #293c3010;
}

.toast.ok .toast-icon {
  color: #57a27b;
  background: #e7f4eb;
}

.toast.error .toast-icon {
  color: #b4647d;
  background: #fae8ef;
}

.toast.warn .toast-icon {
  color: #b38b41;
  background: #faf1dc;
}

.toast.info .toast-icon {
  color: #648baf;
  background: #eaf1f8;
}

.toast-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.25);
}

.toast-text {
  line-height: 1.35;
  word-break: break-all;
}
</style>
