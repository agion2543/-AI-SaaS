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
        <template v-for="section in visibleSections" :key="section.title">
          <div class="menu-section">{{ section.title }}</div>
          <el-menu-item v-for="item in section.items" :key="item.path" :index="item.path">
            <div class="menu-item">
              <strong>{{ item.title }}</strong>
              <span>{{ item.description }}</span>
            </div>
          </el-menu-item>
        </template>
      </el-menu>
      <div v-if="!visibleSections.length" class="empty-permissions">
        <strong>当前账号尚未分配后台权限</strong>
        <span>请联系超管为该账号分配最小必需权限。</span>
      </div>
    </aside>

    <main class="layout-main">
      <header class="layout-header page-card">
        <div>
          <div class="header-title">本地生活商家 AI 运营 SaaS</div>
          <div class="header-subtitle">
            {{ authStore.admin?.display_name || authStore.admin?.username || '管理员' }} · {{ roleName }}
          </div>
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
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const roleName = computed(() => authStore.admin?.role?.name || '未分配角色')
const item = (path, title, description) => ({ path, title, description })
const visibleSections = computed(() => {
  const sections = []
  const isSuperAdmin = authStore.isSuperAdmin
  const has = code => isSuperAdmin || authStore.hasPermission(code)

  if (has('read_only')) {
    const items = [item('/admin/dashboard', '运营总览', '经营概览与平台指标')]
    if (!has('operations')) {
      items.push(
        item('/admin/merchants', '商家资料', '查看商家与门店运营资料'),
        item('/admin/group-buy-links', '团购入口', '查看团购入口与审核记录'),
        item('/admin/users', '顾客数据', '查看顾客资料与消费沉淀')
      )
    }
    sections.push({ title: '只读查询', items })
  }
  if (has('operations')) {
    sections.push({
      title: '运营管理',
      items: [
        item('/admin/todos', '待办中心', '风险、退款、结算、复查入口'),
        item('/admin/merchants', '商家运营', '商家、门店与线索管理'),
        item('/admin/group-buy-links', '团购入口审核', '审核入口、暂停与恢复'),
        item('/admin/users', '顾客运营', '维护顾客状态与资料')
      ]
    })
  }
  if (has('finance')) {
    sections.push({
      title: '财务管理',
      items: [
        item('/admin/packages', '平台套餐', '商家月付与年付方案'),
        item('/admin/subscriptions', '订阅营收', '待确认、收入与到期续费'),
        item('/admin/orders', '订单与支付', '订阅收入、顾客交易与退款'),
        item('/admin/refund-reconciliations', '退款异常核对', '逾期预留、凭证与核验'),
        item('/admin/cards', '卡密工具', '生成和查看兑换卡密')
      ]
    })
  }
  if (has('system_admin')) {
    const items = [
      item('/admin/audit-logs', '操作审计', '审核、退款与配置变更留痕'),
      item('/admin/system', '系统配置', '安全检查、站点和策略配置')
    ]
    if (isSuperAdmin) items.push(item('/admin/platform-costs', '平台成本核算', '确认订阅实收与成本状态'))
    if (isSuperAdmin) items.push(item('/admin/access', '账号与权限', '管理员账号、角色与权限组'))
    sections.push({ title: '系统管理', items })
  } else if (isSuperAdmin) {
    sections.push({ title: '系统管理', items: [item('/admin/access', '账号与权限', '管理员账号、角色与权限组')] })
  }
  return sections.filter(section => section.items.length)
})

const logout = () => {
  authStore.logout()
  router.push('/login')
}

</script>

<style scoped>
.layout-shell{display:grid;grid-template-columns:272px minmax(0,1fr);min-height:100vh;background:radial-gradient(circle at top left,rgba(37,99,235,.16),transparent 24%),radial-gradient(circle at bottom right,rgba(6,182,212,.1),transparent 28%),linear-gradient(135deg,#f8fbff 0%,#eef4fb 100%)}
.layout-sidebar{position:sticky;top:0;height:100vh;overflow:auto;padding:28px 18px;color:#fff;background:radial-gradient(circle at top,rgba(37,99,235,.25),transparent 30%),linear-gradient(180deg,#07182e 0%,#0f2747 100%)}
.brand-block{padding:6px 10px 14px}.brand{font-size:30px;font-weight:900;letter-spacing:-.04em}.brand-subtitle{margin-top:8px;color:#9fb3d9;font-size:13px}.menu{border:none}.menu-section{margin:18px 10px 8px;color:#8fb1d9;font-size:12px;font-weight:900;letter-spacing:.14em}.menu :deep(.el-menu-item){height:58px;margin:5px 0;border-radius:14px}.menu :deep(.el-menu-item:hover){color:#fff!important;background:rgba(37,99,235,.18)}.menu :deep(.el-menu-item.is-active){color:#fff!important;background:linear-gradient(90deg,#2563eb 0%,#06b6d4 100%);box-shadow:0 14px 30px rgba(37,99,235,.3)}.menu-item{display:grid;gap:4px;line-height:1.2}.menu-item strong{font-size:15px}.menu-item span{font-size:12px;opacity:.74}.empty-permissions{display:grid;gap:8px;margin:14px 8px;padding:14px;border:1px solid rgba(148,163,184,.25);border-radius:12px;color:#dbeafe;font-size:13px;line-height:1.5}.empty-permissions span{color:#a9bbd4;font-size:12px}
.layout-main{display:grid;align-content:start;gap:18px;padding:20px 24px 30px}.layout-header{display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 22px;border:1px solid #dce8f5;border-radius:26px;background:rgba(255,255,255,.94)}.header-title{font-size:28px;font-weight:900}.header-subtitle{margin-top:4px;color:#64748b;font-size:13px}.layout-content{display:grid;gap:20px}
@media(max-width:960px){.layout-shell{grid-template-columns:1fr}.layout-sidebar{position:static;height:auto;max-height:none}.layout-header{border-radius:18px}.layout-main{padding:14px}}
</style>
