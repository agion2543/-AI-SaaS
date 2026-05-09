<template>
  <div class="merchant-auth-shell">
    <div class="page-card auth-panel">
      <div class="eyebrow">商家登录</div>
      <h1>进入商家工作台</h1>
      <p>商家账号审核通过后，可在这里登录并完成订阅付费。</p>
      <el-form :model="loginForm">
        <el-form-item>
          <el-input v-model="loginForm.phone" maxlength="11" placeholder="联系人手机号" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="loginForm.password" type="password" show-password placeholder="登录密码" />
        </el-form-item>
        <el-button type="primary" class="full-button" @click="submitLogin">登录商家后台</el-button>
        <el-button text class="full-button register-link" @click="router.push('/merchant/register')">
          还没有账号？申请入驻
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { merchantLogin } from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

const router = useRouter()
const route = useRoute()
const authStore = useMerchantAuthStore()

const loginForm = reactive({
  phone: '',
  password: ''
})

const submitLogin = async () => {
  try {
    const res = await merchantLogin(loginForm)
    authStore.setAuth(res.data)
    ElMessage.success('登录成功')
    router.push(route.query.redirect || '/merchant/dashboard')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '登录失败')
  }
}
</script>

<style scoped>
.merchant-auth-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.18), transparent 26%),
    linear-gradient(145deg, #f7fbff 0%, #ebf4fb 100%);
}

.auth-panel {
  width: min(480px, 100%);
  padding: 30px;
}

.auth-panel h1 {
  margin: 0 0 8px;
  font-size: 34px;
}

.auth-panel p {
  margin: 0 0 20px;
  color: var(--muted);
  line-height: 1.7;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.15em;
  margin-bottom: 8px;
}

.full-button {
  width: 100%;
}

.register-link {
  margin-top: 10px;
}
</style>
