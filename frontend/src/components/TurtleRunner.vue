<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

type Variant = 'roll' | 'walk' | 'hop' | 'flip' | 'spin'

const variants: Variant[] = ['roll', 'walk', 'hop', 'flip', 'spin']

const cycle = ref(0)
const variant = ref<Variant>('walk')
const reverse = ref(false)
const duration = ref(8)
const hidden = ref(true)

let timer: number | undefined

function next() {
  hidden.value = true
  timer = window.setTimeout(() => {
    variant.value = variants[Math.floor(Math.random() * variants.length)]
    reverse.value = Math.random() < 0.5
    duration.value = 6 + Math.random() * 7
    hidden.value = false
    cycle.value++
  }, 800 + Math.random() * 4000)
}

onMounted(next)
onUnmounted(() => {
  if (timer !== undefined) window.clearTimeout(timer)
})
</script>

<template>
  <div class="turtle-track">
    <div
      v-if="!hidden"
      :key="cycle"
      class="turtle-pos"
      :class="{ reverse }"
      :style="{ animationDuration: duration + 's' }"
      @animationend="next"
    >
      <div class="turtle-body" :class="[variant, { reverse }]">
        <svg viewBox="0 0 72 52" width="44" height="32">
          <ellipse cx="20" cy="40" rx="5" ry="4.5" fill="#2dd4bf" stroke="#0d9488" stroke-width="1.5" />
          <ellipse cx="50" cy="40" rx="5" ry="4.5" fill="#2dd4bf" stroke="#0d9488" stroke-width="1.5" />
          <path d="M8 33 L2 37 L9 38 Z" fill="#5eead4" stroke="#0d9488" stroke-width="1.5" stroke-linejoin="round" />
          <path d="M10 34 a24 21 0 0 1 48 0 z" fill="#34d399" stroke="#047857" stroke-width="2" stroke-linejoin="round" />
          <path d="M22 21 a11 9 0 0 1 18 0" fill="none" stroke="#a7f3d0" stroke-width="2.5" stroke-linecap="round" />
          <path d="M17 29 h30" stroke="#a7f3d0" stroke-width="2" stroke-linecap="round" opacity="0.7" />
          <rect x="10" y="34" width="48" height="7" rx="3.5" fill="#fde68a" stroke="#d97706" stroke-width="1.5" />
          <ellipse cx="15" cy="45" rx="6" ry="5" fill="#5eead4" stroke="#0d9488" stroke-width="1.5" />
          <ellipse cx="55" cy="45" rx="6" ry="5" fill="#5eead4" stroke="#0d9488" stroke-width="1.5" />
          <circle cx="62" cy="26" r="9" fill="#5eead4" stroke="#0d9488" stroke-width="2" />
          <circle cx="65" cy="23.5" r="2.6" fill="#fff" />
          <circle cx="65.8" cy="23.5" r="1.4" fill="#134e4a" />
          <circle cx="67.5" cy="29.5" r="2" fill="#f9a8d4" opacity="0.85" />
          <path d="M60 29 q2.5 2 5 0.5" fill="none" stroke="#134e4a" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </div>
    </div>
  </div>
</template>

<style scoped>
.turtle-track {
  position: relative;
  flex: 1;
  height: 100%;
  min-height: 36px;
  overflow: hidden;
  pointer-events: none;
}

.turtle-pos {
  position: absolute;
  bottom: 2px;
  left: -56px;
  animation-name: turtle-move;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

.turtle-pos.reverse {
  animation-name: turtle-move-rev;
}

@keyframes turtle-move {
  from {
    left: -56px;
  }
  to {
    left: calc(100% + 8px);
  }
}

@keyframes turtle-move-rev {
  from {
    left: calc(100% + 8px);
  }
  to {
    left: -56px;
  }
}

.turtle-body {
  transform-origin: 50% 78%;
}

.turtle-body.reverse {
  scale: -1 1;
}

.turtle-body.roll {
  animation: turtle-roll 0.9s linear infinite;
}

.turtle-body.roll.reverse {
  animation-name: turtle-roll-rev;
}

@keyframes turtle-roll {
  to {
    rotate: 360deg;
  }
}

@keyframes turtle-roll-rev {
  to {
    rotate: -360deg;
  }
}

.turtle-body.walk {
  animation: turtle-walk 0.55s ease-in-out infinite;
}

@keyframes turtle-walk {
  0%,
  100% {
    transform: rotate(-5deg) translateY(0);
  }
  50% {
    transform: rotate(5deg) translateY(-2px);
  }
}

.turtle-body.hop {
  animation: turtle-hop 0.6s cubic-bezier(0.34, 1.4, 0.64, 1) infinite;
}

@keyframes turtle-hop {
  0%,
  100% {
    transform: translateY(0);
  }
  40% {
    transform: translateY(-13px);
  }
  60% {
    transform: translateY(-11px);
  }
}

.turtle-body.flip {
  animation: turtle-flip 2.4s ease-in-out infinite;
}

@keyframes turtle-flip {
  0%,
  100% {
    transform: rotate(0deg);
  }
  50% {
    transform: rotate(180deg);
  }
}

.turtle-body.spin {
  animation: turtle-spin 1.1s linear infinite;
}

@keyframes turtle-spin {
  from {
    transform: rotateY(0deg);
  }
  to {
    transform: rotateY(360deg);
  }
}
</style>
