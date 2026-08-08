<script setup lang="ts">
defineProps<{
  type: 'connect' | 'disconnect'
}>()
</script>

<template>
  <div class="flash-overlay" :class="type">
    <div class="flash-wave w1"></div>
    <div class="flash-wave w2"></div>
    <div class="flash-wave w3"></div>
    <div class="flash-center">
      <div class="flash-icon">
        <svg v-if="type === 'connect'" viewBox="0 0 52 52" width="46" height="46">
          <circle class="icon-circle" cx="26" cy="26" r="24" fill="none" stroke="#fff" stroke-width="3" />
          <path class="icon-check" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" d="M14 27l8 8 16-16" />
        </svg>
        <svg v-else viewBox="0 0 52 52" width="46" height="46">
          <circle class="icon-circle" cx="26" cy="26" r="24" fill="none" stroke="#fff" stroke-width="3" />
          <path class="icon-line" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" d="M26 13v13" />
          <path class="icon-arc" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" d="M18.9 17.5a10.5 10.5 0 1 0 14.2 0" />
        </svg>
      </div>
      <div class="flash-text">{{ type === 'connect' ? '已连接' : '已断开' }}</div>
    </div>
  </div>
</template>

<style scoped>
.flash-overlay {
  position: fixed;
  inset: 0;
  z-index: 950;
  display: grid;
  place-items: center;
  pointer-events: none;
  backdrop-filter: blur(5px);
  animation: flash-life 1.6s ease forwards;
}

.flash-overlay.connect {
  background: rgba(230, 250, 238, 0.68);
}

.flash-overlay.disconnect {
  background: rgba(253, 240, 240, 0.68);
}

@keyframes flash-life {
  0% {
    opacity: 1;
  }
  70% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
}

.flash-wave {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 120px;
  height: 120px;
  margin: -60px 0 0 -60px;
  border-radius: 50%;
  opacity: 0;
  animation: wave-expand 1.1s cubic-bezier(0.22, 0.9, 0.36, 1) forwards;
}

.connect .flash-wave {
  background: radial-gradient(circle, rgba(52, 211, 153, 0.35) 0%, rgba(52, 211, 153, 0.12) 55%, transparent 75%);
}

.disconnect .flash-wave {
  background: radial-gradient(circle, rgba(248, 113, 113, 0.3) 0%, rgba(248, 113, 113, 0.1) 55%, transparent 75%);
}

.flash-wave.w2 {
  animation-delay: 0.14s;
}

.flash-wave.w3 {
  animation-delay: 0.28s;
}

@keyframes wave-expand {
  0% {
    transform: scale(0.2);
    opacity: 0.9;
  }
  100% {
    transform: scale(16);
    opacity: 0;
  }
}

.flash-center {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.flash-icon {
  width: 92px;
  height: 92px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  animation: icon-pop 0.6s cubic-bezier(0.34, 1.7, 0.64, 1) 0.1s both;
}

.connect .flash-icon {
  background: linear-gradient(135deg, var(--green-400), var(--green-600));
  box-shadow: 0 14px 40px rgba(16, 185, 129, 0.55), 0 0 0 10px rgba(52, 211, 153, 0.18);
}

.disconnect .flash-icon {
  background: linear-gradient(135deg, var(--red-400), var(--red-500));
  box-shadow: 0 14px 40px rgba(239, 68, 68, 0.5), 0 0 0 10px rgba(248, 113, 113, 0.16);
}

@keyframes icon-pop {
  0% {
    transform: scale(0) rotate(-30deg);
  }
  100% {
    transform: scale(1) rotate(0deg);
  }
}

.icon-circle {
  stroke-dasharray: 151;
  stroke-dashoffset: 151;
  animation: draw 0.5s ease-out 0.35s forwards;
}

.icon-check {
  stroke-dasharray: 48;
  stroke-dashoffset: 48;
  animation: draw 0.4s ease-out 0.6s forwards;
}

.icon-line {
  stroke-dasharray: 20;
  stroke-dashoffset: 20;
  animation: draw 0.3s ease-out 0.5s forwards;
}

.icon-arc {
  stroke-dasharray: 60;
  stroke-dashoffset: 60;
  animation: draw 0.45s ease-out 0.7s forwards;
}

@keyframes draw {
  to {
    stroke-dashoffset: 0;
  }
}

.flash-text {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0.35em;
  text-indent: 0.35em;
  padding: 8px 26px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
  animation: text-rise 0.5s cubic-bezier(0.34, 1.56, 0.64, 1) 0.5s both;
}

.connect .flash-text {
  color: var(--green-700);
}

.disconnect .flash-text {
  color: var(--red-500);
}

@keyframes text-rise {
  0% {
    opacity: 0;
    transform: translateY(16px) scale(0.85);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
</style>
