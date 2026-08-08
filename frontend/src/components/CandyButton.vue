<script setup lang="ts">
import { ref } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'danger' | 'ghost'
    disabled?: boolean
    loading?: boolean
  }>(),
  { variant: 'primary', disabled: false, loading: false },
)

const emit = defineEmits<{ (e: 'click'): void }>()

const btnRef = ref<HTMLButtonElement | null>(null)

function onClick(ev: MouseEvent) {
  if (props.disabled || props.loading) return
  const el = btnRef.value
  if (el) {
    const rect = el.getBoundingClientRect()
    const size = Math.max(rect.width, rect.height)
    const span = document.createElement('span')
    span.className = 'ripple'
    span.style.width = span.style.height = `${size}px`
    span.style.left = `${ev.clientX - rect.left - size / 2}px`
    span.style.top = `${ev.clientY - rect.top - size / 2}px`
    el.appendChild(span)
    window.setTimeout(() => span.remove(), 600)
  }
  emit('click')
}
</script>

<template>
  <button
    ref="btnRef"
    class="btn"
    :class="{ danger: variant === 'danger', ghost: variant === 'ghost' }"
    :disabled="disabled || loading"
    @click="onClick"
  >
    <span v-if="loading" class="spin"></span>
    <slot v-else />
  </button>
</template>
