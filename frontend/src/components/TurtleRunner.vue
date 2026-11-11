<script setup lang="ts">
import { onMounted, onUnmounted, ref, useId } from 'vue'

// One continuous little performance: walk, tuck, roll onto the shell,
// wiggle, roll upright, settle, then turn and walk the other way.
const track = ref<HTMLDivElement | null>(null)
const width = ref(320)
const id = useId().replace(/:/g, '')
const shellId = `${id}-shell`
const skinId = `${id}-skin`
const ground = 57
const cycleDuration = 12.8
// Include the extended head, its stroke and the turning pose, not just the shell.
const edgeInset = 60
const stance = 0.62
const stride = 12 / stance

const clamp = (n: number) => Math.max(0, Math.min(1, n))
const ease = (n: number) => { const t = clamp(n); return t * t * (3 - 2 * t) }
const mix = (a: number, b: number, t: number) => a + (b - a) * t

function leg(hip: number, phase: number, tuck: number, wiggle: number, resting: number) {
  const p = ((phase % 1) + 1) % 1
  // During stance, the foot moves backwards at exactly the body's speed:
  // it stays planted in world space. Only the return stroke lifts the foot.
  const swing = clamp((p - stance) / (1 - stance))
  const step = (p < stance ? 6 - 12 * p / stance : -6 + 12 * ease(swing)) * (1 - resting)
  const lift = (p < stance ? 0 : Math.sin(swing * Math.PI) * 5) * (1 - resting)
  const x = hip + mix(step, hip > 0 ? -4 : 4, tuck) + wiggle
  const y = mix(19 - lift, 8, tuck) - Math.abs(wiggle) * 0.5
  const kneeX = mix(hip, x, 0.55) - 3
  const kneeY = mix(7, y, 0.6)
  return { path: `M ${hip} 6 Q ${kneeX} ${kneeY} ${x} ${y - 2}`, x, y }
}

function poseAt(clock: number, still = false) {
  const lap = Math.floor(clock / cycleDuration)
  const t = clock % cycleDuration
  const direction = lap % 2 === 0 ? 1 : -1
  const room = Math.max(0, width.value - edgeInset * 2)
  const rollDistance = Math.min(120, room * 0.64)
  const walkDistance = room - rollDistance
  const walk = clamp(t / 5.4)
  let distance = walkDistance * walk
  let angle = 0
  let tuck = ease((t - 5.8) / 0.6)
  let wiggle = 0
  let landing = 0

  if (t >= 6.4 && t < 7.7) {
    const roll = ease((t - 6.4) / 1.3)
    angle = 180 * roll
    distance += rollDistance * 0.5 * roll
  } else if (t >= 7.7 && t < 8.8) {
    const progress = (t - 7.7) / 1.1
    angle = 180 + Math.sin(progress * Math.PI * 4) * 9
    distance += rollDistance * 0.5
    tuck = 1 - Math.sin(progress * Math.PI) * 0.65
    wiggle = Math.sin(progress * Math.PI * 8) * 3
  } else if (t >= 8.8) {
    const roll = ease((t - 8.8) / 1.1)
    angle = 180 + 180 * roll
    distance += rollDistance * (0.5 + 0.5 * roll)
    tuck = 1 - ease((t - 9.9) / 0.65)
    landing = Math.sin(clamp((t - 9.9) / 0.65) * Math.PI)
  }

  const walking = t < 5.4 && !still
  const phase = distance / stride
  const gait = walking ? phase : walkDistance / stride
  const rolling = t >= 6.4 && t < 9.9
  const theta = angle * Math.PI / 180
  // The shell's contact radius changes as its domed top turns underneath.
  // Keep that contour on the ground instead of spinning around a floating pivot.
  const support = Math.hypot(25 * Math.sin(theta), 19 * Math.cos(theta)) - 5 * Math.cos(theta)
  const height = rolling ? support : mix(19, 14, tuck) - landing * 1.3
  const turn = ease((t - 11.9) / 0.9)
  const facing = direction * Math.cos(turn * Math.PI)
  const blinkTime = clock % 3.7
  const blink = still ? 1 : 1 - 0.92 * Math.sin(clamp((blinkTime - 3.48) / 0.18) * Math.PI)
  const x = still ? width.value / 2 : direction === 1 ? edgeInset + distance : width.value - edgeInset - distance
  if (still) { tuck = 0; angle = 0; wiggle = 0 }
  const resting = still ? 1 : ease((t - 5.4) / 0.3)
  const legs = [
    leg(-16, gait + 0.5, tuck, wiggle, resting),
    leg(16, gait, tuck, -wiggle, resting),
    leg(-19, gait, tuck, -wiggle, resting),
    leg(17, gait + 0.5, tuck, wiggle, resting),
  ]
  const nod = walking ? Math.sin(phase * Math.PI * 2) * 1.3 : 0
  return {
    actor: `translate(${x} ${still ? ground - 19 : ground - height}) scale(${still ? 1 : facing} 1) rotate(${angle})`,
    head: `translate(${29 - tuck * 21} ${-5 + tuck * 8 + nod}) rotate(${walking ? nod * 2 : -landing * 6}) scale(${1 - tuck * 0.2})`,
    tail: `translate(${tuck * 9} ${-tuck * 3}) rotate(${walking ? Math.sin(phase * Math.PI * 2) * 8 : 0} -22 6)`,
    shell: `scale(${1 + landing * 0.035} ${1 - landing * 0.055})`,
    eye: `translate(10 -3) scale(1 ${blink})`,
    x, legs,
    shadow: rolling ? 23 : 28 - Math.abs(facing) * 2,
    dust: landing * 0.45,
    dustSpread: landing * 8,
  }
}

const pose = ref(poseAt(0))
let frame = 0
let lastTime = 0
let lastPaint = 0
let elapsed = 0
let observer: ResizeObserver | undefined
let motion: MediaQueryList | undefined
let disposed = false

function tick(now: number) {
  if (disposed) return
  if (lastTime) elapsed += Math.min((now - lastTime) / 1000, 0.05)
  lastTime = now
  // Keep the decorative SVG inexpensive; gait and travel use the same clock.
  if (now - lastPaint >= 1000 / 30) {
    pose.value = poseAt(elapsed)
    lastPaint = now
  }
  frame = requestAnimationFrame(tick)
}

function syncMotion() {
  cancelAnimationFrame(frame)
  lastTime = 0
  if (disposed) return
  if (motion?.matches) pose.value = poseAt(0, true)
  else if (!document.hidden) frame = requestAnimationFrame(tick)
}

onMounted(() => {
  motion = window.matchMedia('(prefers-reduced-motion: reduce)')
  motion.addEventListener('change', syncMotion)
  document.addEventListener('visibilitychange', syncMotion)
  observer = new ResizeObserver(([entry]) => {
    if (!entry || disposed) return
    width.value = Math.max(edgeInset * 2, entry.contentRect.width * 64 / 40)
    pose.value = poseAt(elapsed, motion?.matches)
  })
  if (track.value) observer.observe(track.value)
  syncMotion()
})

onUnmounted(() => {
  disposed = true
  cancelAnimationFrame(frame)
  observer?.disconnect()
  motion?.removeEventListener('change', syncMotion)
  document.removeEventListener('visibilitychange', syncMotion)
})
</script>

<template>
  <div ref="track" class="turtle-track" aria-hidden="true">
    <svg :viewBox="`0 0 ${width} 64`" xmlns="http://www.w3.org/2000/svg" class="turtle-scene">
      <defs>
        <linearGradient :id="shellId" x1="0" y1="0" x2="0.7" y2="1">
          <stop stop-color="#6ee7b7" />
          <stop offset="0.52" stop-color="#34c98c" />
          <stop offset="1" stop-color="#07976b" />
        </linearGradient>
        <linearGradient :id="skinId" x1="0" y1="0" x2="0" y2="1">
          <stop stop-color="#d9f99d" />
          <stop offset="1" stop-color="#86d995" />
        </linearGradient>
      </defs>

      <path :d="`M 16 ${ground + 2} H ${width - 16}`" stroke="#10b981" stroke-opacity="0.14" stroke-linecap="round" />
      <ellipse :cx="pose.x" :cy="ground + 1" :rx="pose.shadow" ry="2.7" fill="#047857" opacity="0.12" />
      <g :opacity="pose.dust" fill="#6dbb91">
        <circle :cx="pose.x - 28 - pose.dustSpread" :cy="ground - pose.dustSpread * 0.4" r="1.5" />
        <circle :cx="pose.x + 27 + pose.dustSpread" :cy="ground - pose.dustSpread * 0.6" r="1.7" />
        <circle :cx="pose.x + 33 + pose.dustSpread" :cy="ground - 2" r="1" />
      </g>

      <g :transform="pose.actor" stroke-linecap="round" stroke-linejoin="round">
        <!-- Far legs, neck and tail sit behind the shell as they retract. -->
        <g v-for="i in [0, 1]" :key="`far-${i}`">
          <path :d="pose.legs[i].path" fill="none" stroke="#358c65" stroke-width="7" />
          <ellipse :cx="pose.legs[i].x + 1" :cy="pose.legs[i].y - 1" rx="5" ry="2.4" fill="#358c65" />
        </g>
        <path :transform="pose.tail" d="M -22 3 Q -30 5 -33 10 Q -26 12 -21 9" :fill="`url(#${skinId})`" stroke="#247a55" stroke-width="1.6" />
        <g :transform="pose.head">
          <path d="M -10 7 Q -1 7 1 -1 L 10 2 Q 7 13 -8 14" fill="#98df9d" stroke="#247a55" stroke-width="1.7" />
          <path d="M -2 0 C -2 -10 14 -13 19 -5 C 24 3 18 10 8 10 C 1 10 -3 7 -2 0 Z" :fill="`url(#${skinId})`" stroke="#247a55" stroke-width="1.8" />
          <g :transform="pose.eye">
            <ellipse rx="3.1" ry="3.8" fill="#fffef4" />
            <ellipse cx="0.9" cy="0.3" rx="1.65" ry="2.25" fill="#174b36" />
            <circle cx="1.3" cy="-0.6" r="0.65" fill="white" />
          </g>
          <circle cx="18" cy="-0.5" r="0.8" fill="#247a55" />
          <ellipse cx="12" cy="4.3" rx="3.1" ry="1.7" fill="#f5ada7" opacity="0.7" />
          <path d="M 13 6 Q 17 7 19 4" fill="none" stroke="#247a55" stroke-width="1.2" />
        </g>

        <g :transform="pose.shell">
          <path d="M -25 7 Q -24 15 0 15 Q 24 15 25 7" fill="#f5dfa0" stroke="#64894b" stroke-width="1.8" />
          <path d="M -25 7 C -26 -10 -15 -24 0 -24 C 16 -24 26 -10 25 7 Q 1 14 -25 7 Z" :fill="`url(#${shellId})`" stroke="#176b4c" stroke-width="2" />
          <g fill="none" stroke="#147857" stroke-width="1.35" opacity="0.65">
            <path d="M -8 -17 L 6 -18 L 15 -7 L 6 4 L -9 2 L -15 -8 Z" />
            <path d="M -8 -17 L -11 -21 M 6 -18 L 10 -22 M 15 -7 L 24 -5 M 6 4 L 9 10 M -9 2 L -13 10 M -15 -8 L -23 -6" />
          </g>
          <path d="M -18 -8 Q -13 -20 0 -20" fill="none" stroke="#e2ffe9" stroke-width="2.8" opacity="0.75" />
          <path d="M -23 7 Q 0 13 23 7" fill="none" stroke="#9cf0ba" stroke-width="2" />
        </g>

        <g v-for="i in [2, 3]" :key="`near-${i}`">
          <path :d="pose.legs[i].path" fill="none" stroke="#247a55" stroke-width="8.5" />
          <path :d="pose.legs[i].path" fill="none" stroke="#a3e5a0" stroke-width="5.4" />
          <ellipse :cx="pose.legs[i].x + 1" :cy="pose.legs[i].y - 1.3" rx="5.4" ry="2.7" fill="#b5eaa6" stroke="#247a55" stroke-width="1.4" />
          <path :d="`M ${pose.legs[i].x + 2} ${pose.legs[i].y - 1.5} v 1 M ${pose.legs[i].x + 4} ${pose.legs[i].y - 1.5} v 0.7`" stroke="#568552" stroke-width="0.8" />
        </g>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.turtle-track {
  flex: 1 1 100px;
  min-width: 48px;
  height: 40px;
  overflow: hidden;
  pointer-events: none;
}

.turtle-scene {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
