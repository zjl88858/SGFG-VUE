<script setup lang="ts">
import MainView from './views/MainView.vue'
import OptionsView from './views/OptionsView.vue'
import ToastHost from './components/ToastHost.vue'

const isOptions = window.location.hash.replace(/^#\/?/, '').startsWith('options')
</script>

<template>
  <OptionsView v-if="isOptions" />
  <MainView v-else />
  <ToastHost />
</template>
