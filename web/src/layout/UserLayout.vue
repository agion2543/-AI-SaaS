<template>
  <div class="user-shell">
    <header class="user-header">
      <router-link to="/portal/packages" class="user-brand">SaaS 付费中心</router-link>
      <div class="user-nav">
        <router-link to="/portal/packages">套餐购买</router-link>
        <router-link to="/portal/orders">我的订单</router-link>
        <router-link to="/portal/profile">个人中心</router-link>
        <el-button v-if="store.token" @click="logout">退出登录</el-button>
        <router-link v-else to="/portal/login">用户登录</router-link>
      </div>
    </header>
    <main class="user-main">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'

const router = useRouter()
const store = useUserAuthStore()

const logout = () => {
  store.logout()
  router.push('/portal/login')
}
</script>

<style scoped>
.user-shell {
  min-height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(31, 111, 235, 0.2), transparent 30%),
    linear-gradient(180deg, #f8fbff 0%, #eef3fa 100%);
}

.user-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 32px;
  backdrop-filter: blur(18px);
  background: rgba(255, 255, 255, 0.82);
  border-bottom: 1px solid #e5edf9;
  position: sticky;
  top: 0;
  z-index: 10;
}

.user-brand {
  font-size: 22px;
  font-weight: 800;
  color: #0f172a;
}

.user-nav {
  display: flex;
  gap: 18px;
  align-items: center;
  flex-wrap: wrap;
}

.user-main {
  max-width: 1120px;
  margin: 0 auto;
  padding: 28px 20px 40px;
}

@media (max-width: 760px) {
  .user-header {
    padding: 16px 18px;
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }
}
</style>
