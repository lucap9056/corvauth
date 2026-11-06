import { h } from "vue";
import DefaultTheme from "vitepress/theme";
import ModeNotice from "./ModeNotice.vue";
import ModeSelect from "./ModeSelect.vue";
import "./style.css";

export default {
  extends: DefaultTheme,
  Layout: () =>
    h(DefaultTheme.Layout, null, {
      "sidebar-nav-before": () => h(ModeSelect),
      "doc-before": () => h(ModeNotice),
    }),
};
