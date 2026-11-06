<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { onContentUpdated, useData } from "vitepress";
import { modeLabels } from "./labels";
import { selectMode, selectedMode } from "./mode";

const { lang } = useData();
const labels = computed(() => modeLabels(lang.value));
const message = ref("");

function isHidden(section: Element) {
  return getComputedStyle(section).display === "none";
}

function linkedSection(): Element | null {
  const id = decodeURIComponent(location.hash.slice(1));
  return id ? (document.getElementById(id)?.closest(".mode-scoped") ?? null) : null;
}

async function update() {
  await nextTick();
  const mode = selectedMode.value;
  const sections = [...document.querySelectorAll(".vp-doc .mode-scoped")];
  if (!mode || sections.length === 0) {
    message.value = "";
    return;
  }
  const modeName = labels.value.modes[mode];
  const linked = linkedSection();
  if (sections.every(isHidden)) message.value = labels.value.pageUnavailable(modeName);
  else if (linked && isHidden(linked)) message.value = labels.value.endpointUnavailable(modeName);
  else message.value = "";
}

onMounted(() => window.addEventListener("hashchange", update));
onUnmounted(() => window.removeEventListener("hashchange", update));
onContentUpdated(update);
watch([selectedMode, lang], update);
</script>

<template>
  <div v-if="message" class="mode-notice custom-block warning">
    <p>{{ message }}</p>
    <button type="button" @click="selectMode(null)">{{ labels.showAll }}</button>
  </div>
</template>
