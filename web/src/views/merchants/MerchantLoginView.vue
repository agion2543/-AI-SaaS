<template>
  <div class="merchant-auth-shell">
    <div class="page-card auth-panel">
      <div class="eyebrow">商家登录</div>
      <h1>进入商家工作台</h1>
      <p>商家账号可在这里登录、订阅付费、管理门店商品、订单和 AI 经营分析。</p>

      <el-form :model="loginForm">
        <el-form-item>
          <el-input v-model="loginForm.phone" maxlength="11" placeholder="联系人手机号" />
        </el-form-item>
        <el-segmented v-model="loginMode" :options="loginModeOptions" class="mode-switch" />
        <el-form-item v-if="loginMode === 'password'">
          <el-input v-model="loginForm.password" type="password" show-password placeholder="登录密码" />
        </el-form-item>
        <template v-else>
          <el-form-item>
            <div class="code-row">
              <el-input v-model="loginForm.sms_code" maxlength="6" placeholder="请输入 6 位验证码" />
              <el-button :disabled="loginCountdown > 0 || sendingLoginCode" :loading="sendingLoginCode" @click="sendLoginCode">
                {{ loginCountdown > 0 ? `${loginCountdown}s 后重发` : '发送验证码' }}
              </el-button>
            </div>
          </el-form-item>
          <div class="dev-note">开发/测试环境：验证码固定为 123456，暂未接入真实短信。</div>
        </template>
        <el-button type="primary" class="full-button" :loading="loginSubmitting" @click="submitLogin">登录商家后台</el-button>
        <div class="auth-links">
          <el-button text @click="router.push('/merchant/register')">还没有账号？申请入驻</el-button>
          <el-button text @click="forgotVisible = true">忘记密码</el-button>
        </div>
      </el-form>
    </div>

    <el-dialog v-model="forgotVisible" title="找回商家密码" width="460px">
      <el-form :model="resetForm" label-position="top">
        <el-form-item label="联系人手机号">
          <el-input v-model="resetForm.phone" maxlength="11" placeholder="请输入注册手机号" />
        </el-form-item>
        <el-form-item label="短信验证码">
          <div class="code-row">
            <el-input v-model="resetForm.sms_code" maxlength="6" placeholder="开发环境可用 123456" />
            <el-button :disabled="countdown > 0 || sendingResetCode" :loading="sendingResetCode" @click="sendResetCode">
              {{ countdown > 0 ? `${countdown}s 后重发` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="resetForm.new_password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="resetForm.confirm_password" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="forgotVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetSubmitting" @click="submitResetPassword">重置密码</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  merchantLogin,
  resetMerchantPassword,
  sendMerchantLoginCode,
  sendMerchantPasswordResetCode
} from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

// 与后端 merchant 登录“手机号未注册”文案保持一致，用于引导去注册。
const UNREGISTERED_MESSAGE = '该手机号未注册，请先注册'
const NOT_MERCHANT_MESSAGE = '商家账号不存在'

const router = useRouter()
const route = useRoute()
const authStore = useMerchantAuthStore()

const forgotVisible = ref(false)
const countdown = ref(0)
const loginCountdown = ref(0)
const loginSubmitting = ref(false)
const sendingLoginCode = ref(false)
const sendingResetCode = ref(false)
const resetSubmitting = ref(false)
const loginMode = ref('password')
const loginModeOptions = [
  { label: '密码登录', value: 'password' },
  { label: '验证码登录', value: 'code' }
]
const timers = {}

const loginForm = reactive({
  phone: '',
  password: '',
  sms_code: ''
})

const resetForm = reactive({
  phone: '',
  sms_code: '',
  new_password: '',
  confirm_password: ''
})

const guideToRegister = async () => {
  try {
    await ElMessageBox.confirm('该手机号尚未注册商家账号，是否前往申请入驻？', '手机号未注册', {
      confirmButtonText: '去注册',
      cancelButtonText: '取消'
    })
    router.push('/merchant/register')
  } catch {
    // 用户取消，不做任何事。
  }
}

const submitLogin = async () => {
  if (loginSubmitting.value) return
  loginSubmitting.value = true
  try {
    const payload =
      loginMode.value === 'code'
        ? { phone: loginForm.phone, login_type: 'code', sms_code: loginForm.sms_code }
        : { phone: loginForm.phone, login_type: 'password', password: loginForm.password }
    const res = await merchantLogin(payload)
    authStore.setAuth(res.data)
    ElMessage.success('登录成功')
    router.push(route.query.redirect || '/merchant/dashboard')
  } catch (error) {
    const message = error.response?.data?.message || '登录失败'
    if (message === UNREGISTERED_MESSAGE) {
      await guideToRegister()
    } else {
      ElMessage.error(message)
    }
  } finally {
    loginSubmitting.value = false
  }
}

const startCountdown = (key, target) => {
  target.value = 60
  window.clearInterval(timers[key])
  timers[key] = window.setInterval(() => {
    target.value -= 1
    if (target.value <= 0) {
      window.clearInterval(timers[key])
      timers[key] = null
    }
  }, 1000)
}

const sendLoginCode = async () => {
  if (sendingLoginCode.value || loginCountdown.value > 0) return
  sendingLoginCode.value = true
  try {
    await sendMerchantLoginCode({ phone: loginForm.phone })
    ElMessage.success('验证码已发送，开发/测试环境可直接使用 123456')
    startCountdown('login', loginCountdown)
  } catch (error) {
    const message = error.response?.data?.message || '验证码发送失败'
    if (message === NOT_MERCHANT_MESSAGE) {
      await guideToRegister()
    } else {
      ElMessage.error(message)
    }
  } finally {
    sendingLoginCode.value = false
  }
}

const sendResetCode = async () => {
  if (sendingResetCode.value || countdown.value > 0) return
  sendingResetCode.value = true
  try {
    await sendMerchantPasswordResetCode({ phone: resetForm.phone })
    ElMessage.success('验证码已发送，开发环境可直接使用 123456')
    startCountdown('reset', countdown)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '验证码发送失败')
  } finally {
    sendingResetCode.value = false
  }
}

const submitResetPassword = async () => {
  if (resetSubmitting.value) return
  resetSubmitting.value = true
  try {
    await resetMerchantPassword(resetForm)
    ElMessage.success('密码已重置，请使用新密码登录')
    forgotVisible.value = false
    loginForm.phone = resetForm.phone
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '重置密码失败')
  } finally {
    resetSubmitting.value = false
  }
}

onUnmounted(() => {
  Object.values(timers).forEach((timer) => {
    if (timer) window.clearInterval(timer)
  })
})
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

.mode-switch {
  width: 100%;
  margin-bottom: 18px;
}

.dev-note {
  margin: -6px 0 18px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
}

.full-button {
  width: 100%;
}

.auth-links {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
}

.code-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  width: 100%;
}

@media (max-width: 520px) {
  .auth-links,
  .code-row {
    grid-template-columns: 1fr;
    flex-direction: column;
  }
}
</style>
