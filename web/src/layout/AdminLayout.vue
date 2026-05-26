<template>
  <div class="layout-shell">
    <aside class="layout-sidebar">
      <div class="brand-block">
        <div class="brand">SaaS Admin</div>
        <div class="brand-subtitle">平台运营后台</div>
      </div>

      <el-menu
        router
        :default-active="$route.path"
        class="menu"
        background-color="transparent"
        text-color="#c7d2e5"
        active-text-color="#ffffff"
      >
        <div class="menu-section">平台运营</div>
        <el-menu-item index="/admin/dashboard">
          <div class="menu-item">
            <strong>运营总览</strong>
            <span>收入、交易、风险、巡检</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/todos">
          <div class="menu-item">
            <strong>待办中心</strong>
            <span>风险、退款、结算、复查</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/merchants">
          <div class="menu-item">
            <strong>商家运营</strong>
            <span>入驻、订阅、冻结、经营状态</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/users">
          <div class="menu-item">
            <strong>顾客数据</strong>
            <span>扫码顾客、手机号、消费沉淀</span>
          </div>
        </el-menu-item>

        <div class="menu-section">商业化</div>
        <el-menu-item index="/admin/packages">
          <div class="menu-item">
            <strong>平台套餐</strong>
            <span>卖给商家的月付 / 年付</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/subscriptions">
          <div class="menu-item">
            <strong>订阅营收</strong>
            <span>待确认、收入、到期续费</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/orders">
          <div class="menu-item">
            <strong>订单财务</strong>
            <span>订阅收入与顾客交易流水</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/cards">
          <div class="menu-item">
            <strong>营销工具</strong>
            <span>卡密、兑换、活动预留</span>
          </div>
        </el-menu-item>

        <div class="menu-section">系统</div>
        <el-menu-item index="/admin/audit-logs">
          <div class="menu-item">
            <strong>操作审计</strong>
            <span>审核、退款、开通、冻结留痕</span>
          </div>
        </el-menu-item>
        <el-menu-item index="/admin/system">
          <div class="menu-item">
            <strong>系统配置</strong>
            <span>安全检查、站点、支付、备份</span>
          </div>
        </el-menu-item>
      </el-menu>
    </aside>

    <main class="layout-main">
      <header class="layout-header page-card">
        <div>
          <div class="header-title">本地生活商家 AI 运营 SaaS</div>
          <div class="header-subtitle">平台统一管理商家、订阅、订单、财务和风险。</div>
        </div>
        <el-button type="primary" plain @click="logout">退出登录</el-button>
      </header>

      <section class="layout-content">
        <router-view />
      </section>
    </main>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const logout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout-shell {
  display: grid;
  grid-template-columns: 272px minmax(0, 1fr);
  min-height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.16), transparent 24%),
    radial-gradient(circle at bottom right, rgba(6, 182, 212, 0.1), transparent 28%),
    linear-gradient(135deg, #f8fbff 0%, #eef4fb 100%);
}

.layout-sidebar {
  position: sticky;
  top: 0;
  height: 100vh;
  padding: 28px 18px;
  color: #fff;
  background:
    radial-gradient(circle at top, rgba(37, 99, 235, 0.25), transparent 30%),
    linear-gradient(180deg, #07182e 0%, #0f2747 100%);
}

.brand-block {
  padding: 6px 10px 14px;
}

.brand {
  font-size: 30px;
  font-weight: 900;
  letter-spacing: -0.04em;
}

.brand-subtitle {
  margin-top: 8px;
  color: #9fb3d9;
  font-size: 13px;
}

.menu {
  border: none;
}

.menu-section {
  margin: 18px 10px 8px;
  color: #8fb1d9;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.14em;
}

.menu :deep(.el-menu-item) {
  height: 58px;
  margin: 5px 0;
  border-radius: 14px;
}

.menu :deep(.el-menu-item:hover) {
  color: #fff !important;
  background: rgba(37, 99, 235, 0.18);
}

.menu :deep(.el-menu-item.is-active) {
  color: #fff !important;
  background: linear-gradient(90deg, #2563eb 0%, #06b6d4 100%);
  box-shadow: 0 14px 30px rgba(37, 99, 235, 0.3);
}

.menu-item {
  display: grid;
  gap: 4px;
  line-height: 1.2;
}

.menu-item strong {
  font-size: 15px;
}

.menu-item span {
  font-size: 12px;
  opacity: 0.74;
}

.layout-main {
  display: grid;
  align-content: start;
  gap: 18px;
  padding: 20px 24px 30px;
}

.layout-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 22px;
  border: 1px solid #dce8f5;
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.94);
}

.header-title {
  font-size: 28px;
  font-weight: 900;
}

.header-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 13px;
}

.layout-content {
  display: grid;
  gap: 20px;
}

@media (max-width: 960px) {
  .layout-shell {
    grid-template-columns: 1fr;
  }

  .layout-sidebar {
    position: static;
    height: auto;
  }
}
</style>
