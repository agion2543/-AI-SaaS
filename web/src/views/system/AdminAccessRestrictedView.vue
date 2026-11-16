<template>
  <section class="restricted-page">
    <div class="icon">!</div>
    <p class="eyebrow">ACCESS NOT ASSIGNED</p>
    <h1>当前账号尚未分配后台权限</h1>
    <p>为保护顾客、商家和财务数据，此账号默认不能访问业务模块。请联系平台超管，按岗位需要分配只读、运营、财务或系统管理权限。</p>
    <el-button type="primary" plain @click="logout">退出登录</el-button>
  </section>
</template>
<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
const router = useRouter()
const authStore = useAuthStore()
const logout = () => { authStore.logout(); router.replace('/login') }
</script>
<style scoped>
.restricted-page{min-height:55vh;display:grid;align-content:center;justify-items:center;gap:12px;padding:32px;text-align:center;background:#fff;border:1px solid #e2eaf4;border-radius:20px;box-shadow:0 8px 26px rgba(15,23,42,.045);color:#172b4d}.icon{display:grid;place-items:center;width:44px;height:44px;border-radius:50%;background:#fff7ed;color:#c2410c;font-size:25px;font-weight:800}.eyebrow{margin:0;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.14em}.restricted-page h1{margin:0;font-size:25px}.restricted-page p:not(.eyebrow){max-width:560px;margin:0 0 8px;color:#64748b;line-height:1.7}@media(max-width:600px){.restricted-page{min-height:45vh;padding:22px}.restricted-page h1{font-size:21px}}
</style>
