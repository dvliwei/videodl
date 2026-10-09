# 前端开发说明

前端使用 Vue 3 `<script setup>` 和 Vite。当前 `src/App.vue` 是 VideoDL 页面骨架，分析和下载 API 尚未接入。

按 `docs/DEVELOPMENT_PLAN.md` 逐步创建 `src/features/analyze`、`src/features/downloads` 和 `src/shared`。Wails 绑定由 Go 导出方法生成，位于 `wailsjs`；不要手工修改该目录。

详细产品状态、接口约定和 TRAE 使用方式见仓库根目录的 `docs/PRODUCT_SPEC.md`、`docs/ARCHITECTURE.md` 和 `docs/TRAE_GUIDE.md`。
