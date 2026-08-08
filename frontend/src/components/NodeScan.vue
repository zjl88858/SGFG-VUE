<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  host: string
  attempt: number
}>()

const attemptText = computed(() => (props.attempt > 0 ? props.attempt : 1))
</script>

<template>
  <div class="scan-overlay">
    <div class="scan-stage">
      <div class="scan-badge-wrap">
        <div class="scan-wave w1"></div>
        <div class="scan-wave w2"></div>
        <div class="scan-wave w3"></div>
        <div class="scan-orbit"></div>
        <div class="scan-badge">
          <svg viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round">
            <circle class="sig-center" cx="12" cy="12" r="2.2" fill="#fff" stroke="none" />
            <path class="sig-arc a1" d="M8.5 8.5a5 5 0 0 0 0 7" />
            <path class="sig-arc a2" d="M15.5 8.5a5 5 0 0 1 0 7" />
            <path class="sig-arc a3" d="M5.6 5.6a9 9 0 0 0 0 12.8" />
            <path class="sig-arc a4" d="M18.4 5.6a9 9 0 0 1 0 12.8" />
          </svg>
        </div>
      </div>
      <div class="scan-title">正在分配节点</div>
      <div class="scan-detail" :key="host + attemptText">
        <span class="scan-host">{{ host || '…' }}</span>
        <span class="scan-attempt">第 {{ attemptText }} 次探测</span>
      </div>
      <div class="scan-dots"><i></i><i></i><i></i></div>
    </div>
  </div>
</template>

<style scoped>
.scan-overlay {
  position: fixed;
  inset: 0;
  z-index: 940;
  display: grid;
  place-items: center;
  pointer-events: none;
  background: rgba(230, 250, 238, 0.72);
  backdrop-filter: blur(6px);
}

.scan-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.scan-badge-wrap {
  position: relative;
  width: 96px;
  height: 96px;
  display: grid;
  place-items: center;
}

.scan-wave {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(52, 211, 153, 0.32) 0%, rgba(52, 211, 153, 0.1) 55%, transparent 75%);
  animation: wave-expand 2s cubic-bezier(0.22, 0.9, 0.36, 1) infinite;
}

.scan-wave.w2 {
  animation-delay: 0.6s;
}

.scan-wave.w3 {
  animation-delay: 1.2s;
}

@keyframes wave-expand {
  0% {
    transform: scale(0.55);
    opacity: 0.9;
  }
  100% {
    transform: scale(2.6);
    opacity: 0;
  }
}

.scan-orbit {
  position: absolute;
  inset: -14px;
  border-radius: 50%;
  border: 2px dashed rgba(16, 185, 129, 0.45);
  animation: orbit-spin 6s linear infinite;
}

@keyframes orbit-spin {
  to {
    transform: rotate(360deg);
  }
}

.scan-badge {
  position: relative;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, var(--green-400), var(--green-600));
  box-shadow: 0 14px 40px rgba(16, 185, 129, 0.5), 0 0 0 10px rgba(52, 211, 153, 0.16);
  animation: badge-bob 1.4s ease-in-out infinite;
}

@keyframes badge-bob {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.06);
  }
}

.sig-center {
  animation: center-pulse 1.2s ease-in-out infinite;
  transform-origin: center;
  transform-box: fill-box;
}

@keyframes center-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.4);
    opacity: 0.75;
  }
}

.sig-arc {
  stroke-dasharray: 22;
  stroke-dashoffset: 22;
  animation: arc-draw 1.2s ease-out infinite;
}

.sig-arc.a2 {
  animation-delay: 0.1s;
}

.sig-arc.a3 {
  animation-delay: 0.2s;
}

.sig-arc.a4 {
  animation-delay: 0.3s;
}

@keyframes arc-draw {
  0% {
    stroke-dashoffset: 22;
    opacity: 0.4;
  }
  45% {
    stroke-dashoffset: 0;
    opacity: 1;
  }
  100% {
    stroke-dashoffset: 0;
    opacity: 1;
  }
}

.scan-title {
  font-size: 21px;
  font-weight: 800;
  letter-spacing: 0.3em;
  text-indent: 0.3em;
  color: var(--green-700);
  padding: 8px 26px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
}

.scan-detail {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 16px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.75);
  box-shadow: 0 4px 14px rgba(16, 185, 129, 0.15);
  animation: detail-pop 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes detail-pop {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.scan-host {
  font-size: 13px;
  font-weight: 700;
  font-family: 'Cascadia Mono', Consolas, 'Microsoft YaHei', monospace;
  color: var(--green-800);
}

.scan-attempt {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 10px;
  border-radius: 999px;
  color: #b45309;
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  box-shadow: 0 2px 6px rgba(245, 158, 11, 0.25);
}

.scan-dots {
  display: inline-flex;
  gap: 6px;
}

.scan-dots i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--green-500);
  box-shadow: 0 0 6px rgba(52, 211, 153, 0.7);
  animation: dot-jump 1s ease-in-out infinite;
}

.scan-dots i:nth-child(2) {
  animation-delay: 0.15s;
}

.scan-dots i:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes dot-jump {
  0%,
  100% {
    transform: translateY(0);
    opacity: 0.35;
  }
  50% {
    transform: translateY(-6px);
    opacity: 1;
  }
}
</style>
