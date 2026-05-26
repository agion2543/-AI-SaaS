<template>
  <div class="merchant-login">
    <!-- 背景装饰 -->
    <div class="login-bg">
      <div class="logo-area">
        <van-image
          width="80"
          height="80"
          round
          fit="cover"
          :src="logoUrl"
          class="logo"
        />
        <h1 class="app-name">商家管理系统</h1>
        <p class="app-desc">本地生活商家 AI 运营 SaaS</p>
      </div>
    </div>

    <!-- 登录表单 -->
    <div class="login-form">
      <van-form @submit="onLogin">
        <van-cell-group inset>
          <van-field
            v-model="form.phone"
            name="phone"
            label="手机号"
            placeholder="请输入11位手机号"
            maxlength="11"
            type="tel"
            :rules="[
              { required: true, message: '请输入手机号' },
              { pattern: /^1[3-9]\d{9}$/, message: '手机号必须为11位数字' }
            ]"
          />
          <van-field
            v-model="form.password"
            type="password"
            name="password"
            label="密码"
            placeholder="请输入密码（至少6位）"
            :rules="[
              { required: true, message: '请输入密码' },
              { validator: (val) => val.length >= 6 || '密码至少6位' }
            ]"
          />
        </van-cell-group>

        <!-- 开发模式提示 -->
        <div v-if="isDev" class="dev-notice">
          <van-notice-bar
            left-icon="info-o"
            background="#ecf5ff"
            color="#1989fa"
          >
            💡 开发模式：先注册账号 → 用 手机号+密码 登录即可
          </van-notice-bar>
        </div>

        <div class="form-submit">
          <van-button
            round
            block
            type="primary"
            native-type="submit"
            :loading="loginLoading"
          >
            登 录
          </van-button>
        </div>
      </van-form>

      <div class="form-links">
        <router-link to="/m/merchant/register" class="link-btn">
          还没有账号？<b>立即注册</b>
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showSuccessToast, showFailToast } from 'vant'
import { merchantLogin } from '@/api/modules'

const route = useRoute()
const router = useRouter()

const logoUrl = 'https://via.placeholder.com/80x80?text=Logo'
const isDev = import.meta.env.DEV ?? true
const loginLoading = ref(false)

const form = ref({
  phone: '',
  password: ''
})

async function onLogin() {
  loginLoading.value = true
  try {
    const res = await merchantLogin({
      phone: form.value.phone,
      password: form.value.password,
      login_type: 'password'
    })

    localStorage.setItem('merchant_token', res.data.token)
    localStorage.setItem('merchant_profile', JSON.stringify(res.data.merchant))

    showSuccessToast('登录成功')

    const redirect = route.query.redirect || '/m/merchant/home'
    router.replace(redirect)
  } catch (e) {
    const errMsg = e.response?.data?.message
      || e.message
      || '登录失败，请检查手机号和密码'
    console.error('[登录失败]', e.response?.data || e.message || e)
    showFailToast(errMsg)
  } finally {
    loginLoading.value = false
  }
}
</script>

<style scoped>
.merchant-login {
  min-height: 100vh;
  background: #f7f8fa;
}

.login-bg {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 60px 20px 40px;
  color: white;
}

.logo-area {
  text-align: center;
}

.logo {
  margin-bottom: 16px;
}

.app-name {
  font-size: 24px;
  font-weight: bold;
  margin: 0 0 8px;
}

.app-desc {
  font-size: 14px;
  opacity: 0.9;
  margin: 0;
}

.login-form {
  background: white;
  border-radius: 16px 16px 0 0;
  padding: 24px 16px;
  margin-top: -20px;
  min-height: calc(100vh - 200px);
}

.form-submit {
  padding: 28px 16px 0;
}

.form-links {
  text-align: center;
  margin-top: 24px;
  font-size: 14px;
  color: #666;
}

.form-links .link-btn {
  color: #667eea;
  text-decoration: none;
}

.dev-notice {
  margin: 12px 16px;
}
</style>
