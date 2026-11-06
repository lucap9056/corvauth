---
layout: page
---

<script setup>
import { onMounted } from "vue";
import { withBase } from "vitepress";

onMounted(() => {
  const prefersChinese = navigator.languages.some((lang) => lang.toLowerCase().startsWith("zh"));
  location.replace(withBase(prefersChinese ? "/zh-TW/" : "/en/"));
});
</script>

- [English](./en/index.md)
- [繁體中文](./zh-TW/index.md)
