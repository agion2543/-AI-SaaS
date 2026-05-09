<template>
  <div class="layout-shell">
    <aside class="layout-sidebar">
      <div class="brand-block">
        <div class="brand">SaaS Admin</div>
        <div class="brand-subtitle">平台方运营后台</div>
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
            <span>收入、交易、风控、巡检</span>
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
            <span>站点、支付、公告、备案</span>
          </div>
        </el-menu-item>
      </el-menu>
    </aside>

    <main class="layout-main">
      <header class="layout-header page-card">
        <div>
          <div class="header-title">本地生活商家 AI 运营 SaaS</div>
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
    radial-gradient(circle at top left, rgba(59, 130, 246, 0.14), transparent 24%),
    linear-gradient(135deg, #f8fbff 0%, #eef3f9 100%);
}

.layout-sidebar {
  position: sticky;
  top: 0;
  height: 100vh;
  padding: 28px 18px;
  background:
    radial-gradient(circle at top, rgba(59, 130, 246, 0.18), transparent 30%),
    linear-gradient(180deg, #0f172a 0%, #16233b 100%);
  color: #fff;
}

.brand-block { padding: 6px 10px 14px; }
.brand { font-size: 30px; font-weight: 900; }
.brand-subtitle { margin-top: 8px; color: #9fb3d9; font-size: 13px; }
.menu { border: none; }
.menu-section { margin: 18px 10px 8px; color: #8fb1d9; font-size: 12px; font-weight: 900; letter-spacing: 0.14em; }
.menu :deep(.el-menu-item) { height: 58px; margin: 5px 0; border-radius: 10px; }
.menu :deep(.el-menu-item:hover) { background: rgba(59, 130, 246, 0.14); color: #fff !important; }
.menu :deep(.el-menu-item.is-active) { background: linear-gradient(90deg, #2563eb 0%, #3b82f6 100%); color: #fff !important; box-shadow: 0 12px 24px rgba(37, 99, 235, 0.24); }
.menu-item { display: grid; gap: 4px; line-height: 1.2; }
.menu-item strong { font-size: 15px; }
.menu-item span { font-size: 12px; opacity: 0.74; }

.layout-main { display: grid; gap: 18px; padding: 18px 22px 28px; align-content: start; }
.layout-header { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 18px 22px; border: 1px solid #dbeafe; background: rgba(255, 255, 255, 0.94); }
.header-title { font-size: 28px; font-weight: 900; }
.layout-content { display: grid; gap: 20px; }

@media (max-width: 960px) {
  .layout-shell { grid-template-columns: 1fr; }
  .layout-sidebar { position: static; height: auto; }
}
</style>
