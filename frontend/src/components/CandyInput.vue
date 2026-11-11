<script setup lang="ts">
defineProps<{
  modelValue: string
  placeholder?: string
  type?: string
  disabled?: boolean
}>()

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

function onInput(ev: Event) {
  emit('update:modelValue', (ev.target as HTMLInputElement).value)
}
</script>

<template>
  <input
    class="candy-input"
    :type="type ?? 'text'"
    :value="modelValue"
    :placeholder="placeholder"
    :disabled="disabled"
    @input="onInput"
  />
</template>

<style scoped>
.candy-input {
  width: 100%;
  padding: 9px 14px;
  border-radius: 12px;
  min-width: 0;
  border: 1px solid #cce5d5;
  background: rgba(255, 255, 255, 0.9);
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.15s ease, opacity 0.4s ease;
}

.candy-input::placeholder {
  color: #9ca3af;
  font-weight: 400;
}

.candy-input:hover:not(:disabled) {
  border-color: var(--green-300);
}

.candy-input:focus {
  border-color: var(--green-400);
  box-shadow: 0 0 0 4px rgba(52, 211, 153, 0.18);
}

.candy-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
