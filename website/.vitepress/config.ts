import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type MarkdownIt from "markdown-it";
import { defineConfig, type DefaultTheme } from "vitepress";
import { deploymentModes, modeQueryParam, modeStorageKey } from "./modes";
import { endpointAnchor, endpointHeading, loadSpec, operationModes, renderEndpoints } from "./openapi";

const repoRoot = fileURLToPath(new URL("../..", import.meta.url));
const docsRoot = path.join(repoRoot, "docs");
const websiteModules = toPosix(fileURLToPath(new URL("../node_modules", import.meta.url)));
const repoURL = "https://github.com/lucap9056/corvauth";
const base = "/corvauth/";
const spec = loadSpec(path.join(docsRoot, "public", "openapi.yaml"));
const sections = [
  { key: "start", dir: "", pages: ["index", "quick-start", "reference"] },
  { key: "topics", dir: "topics", pages: ["deployment-modes", "oauth2", "tokens", "status-codes", "logging", "configuration"] },
  { key: "resources", dir: "resources", pages: ["health", "session", "user", "verify"] },
] as const;

const sectionTitles: Record<string, Record<(typeof sections)[number]["key"], string>> = {
  en: { start: "Getting Started", topics: "Topics", resources: "API" },
  "zh-TW": { start: "入門", topics: "主題", resources: "API" },
};

function toPosix(p: string): string {
  return p.split(path.sep).join("/");
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{M}\p{N}\p{Pc}\- ]/gu, "")
    .replace(/ /g, "-");
}

function pageTitle(file: string): string {
  const heading = fs.readFileSync(file, "utf8").match(/^# (.+)$/m);
  if (!heading) throw new Error(`missing title: ${file}`);
  return heading[1].replace(/`/g, "");
}

function sidebarTitle(file: string): string {
  return pageTitle(file).replace(/ API$/, "");
}

function documentedEndpoints(file: string) {
  return [...fs.readFileSync(file, "utf8").matchAll(new RegExp(endpointHeading, "gm"))].map(([, name, method, urlPath]) => ({
    name,
    method,
    urlPath,
  }));
}

function assertEndpointsCovered(lang: string, documented: Set<string>) {
  for (const [urlPath, operations] of Object.entries<Record<string, unknown>>(spec.paths)) {
    for (const method of Object.keys(operations)) {
      const endpoint = `${method.toUpperCase()} ${urlPath}`;
      if (!documented.has(endpoint)) throw new Error(`docs/${lang}/resources does not document ${endpoint}`);
    }
  }
}

function sidebar(lang: string): DefaultTheme.SidebarItem[] {
  const documented = new Set<string>();
  const groups = sections.map(({ key, dir, pages }) => {
    const items = pages.map((page) => {
      const file = path.join(docsRoot, lang, dir, `${page}.md`);
      const link = `/${lang}/${dir ? `${dir}/` : ""}${page === "index" ? "" : page}`;
      const endpoints = documentedEndpoints(file).map(({ name, method, urlPath }) => {
        documented.add(`${method} ${urlPath}`);
        return { text: name, link: `${link}#${endpointAnchor(method, urlPath)}` };
      });
      return { text: sidebarTitle(file), link, ...(endpoints.length ? { items: endpoints } : {}) };
    });
    return { text: sectionTitles[lang][key], items };
  });
  assertEndpointsCovered(lang, documented);
  return groups;
}

function hiddenInMode(mode: string): string[] {
  const resources = sections.find(({ dir }) => dir === "resources")!;
  return resources.pages.flatMap((page) => {
    const file = path.join(docsRoot, "en", resources.dir, `${page}.md`);
    const endpoints = documentedEndpoints(file).map(({ method, urlPath }) => ({
      anchor: endpointAnchor(method, urlPath),
      available: operationModes(spec, method, urlPath).includes(mode as never),
    }));
    const hiddenEndpoints = endpoints
      .filter(({ available }) => !available)
      .flatMap(({ anchor }) => [
        `.VPSidebarItem:has(> .item a[href$="#${anchor}"])`,
        `.VPDocOutlineItem li:has(> a[href="#${anchor}"])`,
      ]);
    const pageHref = `/${resources.dir}/${page}`;
    const hiddenPage = endpoints.some(({ available }) => available)
      ? []
      : [`.VPSidebarItem:has(> .item a[href$="${pageHref}"])`, `.VPDocFooter .pager:has(> a[href$="${pageHref}"])`];
    return [...hiddenEndpoints, ...hiddenPage];
  });
}

function modeStyle(): string {
  return deploymentModes
    .map((mode) => {
      const selectors = [`.mode-scoped:not([data-modes~="${mode}"])`, ...hiddenInMode(mode)];
      const emptyOutline = `.VPDoc:has(.mode-scoped):not(:has(.mode-scoped[data-modes~="${mode}"])) .VPDocAsideOutline`;
      return `html[data-mode="${mode}"] :is(${[...selectors, emptyOutline].join(", ")}) { display: none; }`;
    })
    .join("\n");
}

const applySavedMode = `(() => {
  const modes = ${JSON.stringify(deploymentModes)};
  try {
    const requested = new URLSearchParams(location.search).get(${JSON.stringify(modeQueryParam)});
    if (requested !== null) {
      if (modes.includes(requested)) localStorage.setItem(${JSON.stringify(modeStorageKey)}, requested);
      else localStorage.removeItem(${JSON.stringify(modeStorageKey)});
    }
    const saved = localStorage.getItem(${JSON.stringify(modeStorageKey)});
    if (modes.includes(saved)) document.documentElement.dataset.mode = saved;
  } catch {}
})();`;

function endpoints(md: MarkdownIt) {
  md.core.ruler.after("normalize", "endpoints", (state) => {
    if (/^[^/]+\/resources\//.test(state.env.relativePath ?? "")) state.src = renderEndpoints(spec, state.src);
  });
}

function linksOutsideDocs(md: MarkdownIt) {
  md.core.ruler.after("inline", "links-outside-docs", (state) => {
    const pagePath = state.env.realPath ?? state.env.path;
    if (!pagePath) return;
    const pageDir = path.dirname(pagePath);
    for (const token of state.tokens.flatMap((block) => block.children ?? [])) {
      const href = token.type === "link_open" ? token.attrGet("href") : null;
      if (!href || /^([a-z][a-z0-9+.-]*:|#|\/)/i.test(href)) continue;
      const target = toPosix(path.relative(repoRoot, path.resolve(pageDir, href)));
      if (target.startsWith("docs/public/")) token.attrSet("href", target.slice("docs/public".length));
      else if (!target.startsWith("docs/")) token.attrSet("href", `${repoURL}/blob/main/${target}`);
    }
  });
}

export default defineConfig({
  title: "Corvauth",
  base,
  srcDir: "../docs",
  cleanUrls: true,
  head: [
    ["script", {}, applySavedMode],
    ["style", {}, modeStyle()],
  ],
  markdown: {
    anchor: { slugify },
    config: (md) => {
      endpoints(md);
      linksOutsideDocs(md);
    },
  },
  vite: { resolve: { alias: [{ find: /^vue(\/server-renderer)?$/, replacement: `${websiteModules}/vue$1` }] } },
  themeConfig: {
    search: { provider: "local" },
    outline: { level: [2, 3] },
    socialLinks: [{ icon: "github", link: repoURL }],
  },
  locales: {
    en: { label: "English", lang: "en", link: "/en/", themeConfig: { sidebar: sidebar("en") } },
    "zh-TW": {
      label: "繁體中文",
      lang: "zh-TW",
      link: "/zh-TW/",
      themeConfig: {
        sidebar: sidebar("zh-TW"),
        outline: { level: [2, 3], label: "本頁內容" },
        docFooter: { prev: "上一頁", next: "下一頁" },
      },
    },
  },
});
