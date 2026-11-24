<template>
  <main class="merchant-mine">
    <van-nav-bar title="我的" />

    <section class="mine-section" aria-label="账户与服务">
      <van-cell-group inset>
        <van-cell
          title="账户与设置"
          label="商家资料、账号安全与其他设置"
          is-link
          to="/m/merchant/settings"
        />
        <van-cell
          title="订阅与业务状态"
          label="查看服务状态、续费与访问限制"
          is-link
          to="/m/merchant/subscription"
        />
      </van-cell-group>
    </section>
  </main>
</template>

<style scoped>
.merchant-mine {
  min-height: 100vh;
  padding: 12px 0 76px;
  background: #f7f8fa;
}

.mine-section {
  margin-top: 12px;
}
</style>
