<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  label: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'change', v: boolean): void
}>()

function toggle() {
  if (props.disabled) return
  const v = !props.modelValue
  emit('update:modelValue', v)
  emit('change', v)
}
</script>

<template>
  <div class="toggle-row" :class="{ disabled }" @click="toggle">
    <span class="toggle-label">{{ label }}</span>
    <span class="toggle-track" :class="{ on: modelValue }">
      <span class="toggle-knob"></span>
    </span>
  </div>
</template>

<style scoped>
.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 4px;
  cursor: pointer;
  border-radius: 10px;
  transition: background 0.2s ease, opacity 0.4s ease;
}

.toggle-row .toggle-track,
.toggle-row .toggle-label {
  transition: opacity 0.4s ease;
}

.toggle-row:hover:not(.disabled) {
  background: rgba(167, 243, 208, 0.25);
}

.toggle-row.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.toggle-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--ink);
}

.toggle-track {
  flex: none;
  width: 46px;
  height: 26px;
  border-radius: 999px;
  background: #d7e4dc;
  position: relative;
  transition: background 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.08);
}

.toggle-track.on {
  background: linear-gradient(135deg, var(--green-400), var(--green-500));
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.06), 0 0 10px rgba(52, 211, 153, 0.5);
}

.toggle-knob {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
  transition: transform 0.28s cubic-bezier(0.34, 1.8, 0.64, 1);
}

.toggle-track.on .toggle-knob {
  transform: translateX(20px);
}

.toggle-row:active:not(.disabled) .toggle-knob {
  transform: scale(0.85);
}

.toggle-row:active:not(.disabled) .toggle-track.on .toggle-knob {
  transform: translateX(20px) scale(0.85);
}
</style>
