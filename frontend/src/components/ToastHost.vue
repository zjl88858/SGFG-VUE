<script setup lang="ts">
import { toasts } from '../toast'
</script>

<template>
  <div class="toast-host">
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
  top: 16px;
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
  min-width: 200px;
  max-width: 340px;
  padding: 10px 16px;
  border-radius: 14px;
  font-size: 13.5px;
  font-weight: 700;
  color: #fff;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.16);
}

.toast.ok {
  background: linear-gradient(135deg, var(--green-400), var(--green-600));
}

.toast.error {
  background: linear-gradient(135deg, var(--red-400), var(--red-500));
}

.toast.warn {
  background: linear-gradient(135deg, var(--yellow-300), #f59e0b);
}

.toast.info {
  background: linear-gradient(135deg, #93c5fd, #3b82f6);
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
