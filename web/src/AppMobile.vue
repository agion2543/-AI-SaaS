<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script setup>
// AppMobile.vue - 移动端根组件
// 移动端 App 入口，简洁的 router-view
</script>

<style>
/* 移动端全局样式 */
html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  background: #f7f8fa;
}

/* 修复 iOS 安全区域 */
body {
  padding: env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);
}

/* 隐藏桌面端元素 */
.admin-sidebar,
.admin-header,
.desktop-only {
  display: none !important;
}

/* 移动端友好的滚动 */
.scroll-touch {
  -webkit-overflow-scrolling: touch;
  overflow-y: auto;
}

/* 移动端按钮优化 */
@media screen and (max-width: 768px) {
  .van-button {
    min-height: 44px;
    line-height: 44px;
  }

  .van-cell {
    min-height: 48px;
  }

  .van-field__control {
    min-height: 44px;
  }
}
</style>
