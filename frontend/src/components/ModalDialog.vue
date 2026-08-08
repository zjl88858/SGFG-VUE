<script setup lang="ts">
import CandyButton from './CandyButton.vue'

defineProps<{
  open: boolean
  title: string
  lines: string[]
  danger?: boolean
}>()

const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <Transition name="fade">
    <div v-if="open" class="modal-mask" @click.self="emit('close')">
      <Transition name="pop" appear>
        <div class="modal-card">
          <div class="modal-title" :class="{ danger }">{{ title }}</div>
          <div class="modal-body">
            <p v-for="(line, i) in lines" :key="i">{{ line }}</p>
          </div>
          <div class="modal-footer">
            <CandyButton style="padding: 8px 26px" @click="emit('close')">知道了</CandyButton>
          </div>
        </div>
      </Transition>
    </div>
  </Transition>
</template>

<style scoped>
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 900;
  display: grid;
  place-items: center;
  background: rgba(6, 95, 70, 0.18);
  backdrop-filter: blur(3px);
}

.modal-card {
  width: 400px;
  max-width: calc(100vw - 48px);
  background: rgba(255, 255, 255, 0.96);
  border-radius: 20px;
  border: 1px solid var(--card-border);
  box-shadow: 0 20px 50px rgba(6, 95, 70, 0.25);
  padding: 22px 24px 18px;
}

.modal-title {
  font-size: 16px;
  font-weight: 800;
  color: var(--green-700);
  margin-bottom: 12px;
}

.modal-title.danger {
  color: var(--red-500);
}

.modal-body {
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--ink);
  margin-bottom: 16px;
  user-select: text;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
}
</style>
