<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  label: string
  description?: string
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
  <button type="button" class="toggle-row" role="switch" :aria-checked="modelValue" :aria-label="label" :class="{ disabled }" :disabled="disabled" @click="toggle">
    <span class="toggle-copy"><span class="toggle-label">{{ label }}</span><span v-if="description" class="toggle-description">{{ description }}</span></span>
    <span class="toggle-track" :class="{ on: modelValue }" aria-hidden="true">
      <span class="toggle-knob"></span>
    </span>
  </button>
</template>

<style scoped>
.toggle-row {
  width: 100%;
  border: 0;
  background: transparent;
  text-align: left;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 4px;
  cursor: pointer;
  border-radius: 10px;
  transition: background 0.2s ease, opacity 0.4s ease;
}

.toggle-copy { display: flex; flex-direction: column; gap: 4px; }
.toggle-description { font-size: 10px; color: var(--muted); line-height: 1.5; }

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
  font-size: 13px;
  font-weight: 400;
  color: var(--ink);
}

.toggle-track {
  flex: none;
  width: 40px;
  height: 23px;
  border-radius: 999px;
  background: #d7e4dc;
  position: relative;
  transition: background 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.08);
}

.toggle-track.on {
  background: #78bf97;
  box-shadow: none;
}

.toggle-knob {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 17px;
  height: 17px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
  transition: transform 0.28s cubic-bezier(0.34, 1.8, 0.64, 1);
}

.toggle-track.on .toggle-knob {
  transform: translateX(17px);
}

.toggle-row:active:not(.disabled) .toggle-knob {
  transform: scale(0.85);
}

.toggle-row:active:not(.disabled) .toggle-track.on .toggle-knob {
  transform: translateX(17px) scale(0.85);
}
</style>
