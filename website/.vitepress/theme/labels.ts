import type { DeploymentMode } from "../modes";

interface ModeLabels {
  select: string;
  all: string;
  modes: Record<DeploymentMode, string>;
  pageUnavailable: (mode: string) => string;
  endpointUnavailable: (mode: string) => string;
  showAll: string;
}

const modes = { stateless: "Stateless proxy", managed: "Managed users", external: "External users" };

const labels: Record<string, ModeLabels> = {
  en: {
    select: "Deployment mode",
    all: "All modes",
    modes,
    pageUnavailable: (mode) => `No endpoint on this page is available in ${mode} mode.`,
    endpointUnavailable: (mode) => `The linked endpoint is not available in ${mode} mode.`,
    showAll: "Show all modes",
  },
  "zh-TW": {
    select: "部署模式",
    all: "全部模式",
    modes,
    pageUnavailable: (mode) => `此頁的 endpoint 在 ${mode} 模式下皆無效。`,
    endpointUnavailable: (mode) => `連結的 endpoint 在 ${mode} 模式下無效。`,
    showAll: "顯示全部模式",
  },
};

export function modeLabels(lang: string): ModeLabels {
  return labels[lang] ?? labels.en;
}
