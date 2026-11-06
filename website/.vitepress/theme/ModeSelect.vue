<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useData } from "vitepress";
import { deploymentModes, isDeploymentMode } from "../modes";
import { modeLabels } from "./labels";
import { loadMode, selectMode, selectedMode } from "./mode";

const { lang } = useData();
const labels = computed(() => modeLabels(lang.value));

const value = computed({
  get: () => selectedMode.value ?? "",
  set: (mode: string) => selectMode(isDeploymentMode(mode) ? mode : null),
});

onMounted(loadMode);
</script>

<template>
  <label class="mode-select">
    <span class="mode-select-label">{{ labels.select }}</span>
    <select v-model="value">
      <option value="">{{ labels.all }}</option>
      <option v-for="mode in deploymentModes" :key="mode" :value="mode">{{ labels.modes[mode] }}</option>
    </select>
  </label>
</template>
