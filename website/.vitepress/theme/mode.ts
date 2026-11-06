import { ref } from "vue";
import { isDeploymentMode, modeStorageKey, type DeploymentMode } from "../modes";

export const selectedMode = ref<DeploymentMode | null>(null);

export function loadMode() {
  const saved = document.documentElement.dataset.mode;
  selectedMode.value = isDeploymentMode(saved) ? saved : null;
}

export function selectMode(mode: DeploymentMode | null) {
  selectedMode.value = mode;
  if (mode) document.documentElement.dataset.mode = mode;
  else delete document.documentElement.dataset.mode;
  try {
    if (mode) localStorage.setItem(modeStorageKey, mode);
    else localStorage.removeItem(modeStorageKey);
  } catch {}
}
