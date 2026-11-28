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
        <!-- 登录方式切换 -->
        <div class="mode-switch">
          <van-button
            size="small"
            :type="loginMode === 'password' ? 'primary' : 'default'"
            @click="loginMode = 'password'"
          >
            密码登录
          </van-button>
          <van-button
            size="small"
            :type="loginMode === 'code' ? 'primary' : 'default'"
            @click="loginMode = 'code'"
          >
            验证码登录
          </van-button>
        </div>

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
            v-if="loginMode === 'password'"
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
          <van-field
            v-else
            v-model="form.code"
            name="code"
            label="验证码"
            placeholder="请输入6位验证码"
            maxlength="6"
            type="tel"
            :rules="[
              { required: true, message: '请输入验证码' },
              { pattern: /^\d{6}$/, message: '验证码为6位数字' }
            ]"
          >
            <template #button>
              <van-button size="small" type="primary" :disabled="loginCountdown > 0 || sendingLoginCode" :loading="sendingLoginCode" @click.prevent="sendLoginCode">
                {{ loginCodeText }}
              </van-button>
            </template>
          </van-field>
        </van-cell-group>

        <!-- 开发模式提示 -->
        <div v-if="isDev" class="dev-notice">
          <van-notice-bar
            left-icon="info-o"
            background="#ecf5ff"
            color="#1989fa"
          >
            <template v-if="loginMode === 'code'">
              💡 开发模式：验证码固定为 <b>123456</b>，暂未接入真实短信
            </template>
            <template v-else>
              💡 开发模式：先注册账号 → 用 手机号+密码 登录即可
            </template>
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
        <a class="link-btn" @click.prevent="openForgotPopup">忘记密码</a>
      </div>
    </div>

    <!-- 忘记密码弹窗 -->
    <van-popup v-model:show="forgotVisible" position="bottom" round style="height: 72%;">
      <div class="forgot-popup">
        <h3>找回商家密码</h3>
        <van-form @submit="onResetPassword">
          <van-cell-group inset>
            <van-field
              v-model="resetForm.phone"
              name="phone"
              label="手机号"
              placeholder="请输入注册手机号"
              maxlength="11"
              type="tel"
              :rules="[
                { required: true, message: '请输入手机号' },
                { pattern: /^1[3-9]\d{9}$/, message: '手机号必须为11位数字' }
              ]"
            />
            <van-field
              v-model="resetForm.sms_code"
              name="sms_code"
              label="验证码"
              placeholder="开发环境可用 123456"
              maxlength="6"
              type="tel"
              :rules="[
                { required: true, message: '请输入验证码' },
                { pattern: /^\d{6}$/, message: '验证码为6位数字' }
              ]"
            >
              <template #button>
                <van-button size="small" type="primary" :disabled="resetCountdown > 0 || sendingResetCode" :loading="sendingResetCode" @click.prevent="sendResetCode">
                  {{ resetCodeText }}
                </van-button>
              </template>
            </van-field>
            <van-field
              v-model="resetForm.new_password"
              type="password"
              name="new_password"
              label="新密码"
              placeholder="至少 6 位"
              :rules="[
                { required: true, message: '请输入新密码' },
                { validator: (val) => val.length >= 6 || '新密码至少6位' }
              ]"
            />
            <van-field
              v-model="resetForm.confirm_password"
              type="password"
              name="confirm_password"
              label="确认密码"
              placeholder="再次输入新密码"
              :rules="[
                { required: true, message: '请确认新密码' },
                { validator: (val) => val === resetForm.new_password || '两次密码不一致' }
              ]"
            />
          </van-cell-group>
          <div class="forgot-submit">
            <van-button round block type="primary" native-type="submit" :loading="resetLoading">
              重置密码
            </van-button>
          </div>
        </van-form>
        <p class="forgot-tip">开发/测试环境：验证码固定为 123456，暂未接入真实短信。</p>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { computed, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showSuccessToast, showFailToast, showToast } from 'vant'
import {
  merchantLogin,
  resetMerchantPassword,
  sendMerchantLoginCode,
  sendMerchantPasswordResetCode
} from '@/api/modules'

const route = useRoute()
const router = useRouter()

const logoUrl = 'https://via.placeholder.com/80x80?text=Logo'
const isDev = import.meta.env.DEV ?? true
const loginLoading = ref(false)
const loginMode = ref('password')

// 与后端 merchant 登录“手机号未注册”文案保持一致，用于引导去注册。
const UNREGISTERED_MESSAGE = '该手机号未注册，请先注册'
const NOT_MERCHANT_MESSAGE = '商家账号不存在'

const form = ref({
  phone: '',
  password: '',
  code: ''
})

const loginCountdown = ref(0)
const sendingLoginCode = ref(false)
let loginTimer = null
const loginCodeText = computed(() => {
  if (loginCountdown.value > 0) {
    return `${loginCountdown.value}s 后重发`
  }
  return '发送验证码'
})

const forgotVisible = ref(false)
const resetLoading = ref(false)
const resetCountdown = ref(0)
const sendingResetCode = ref(false)
let resetTimer = null
const resetForm = ref({
  phone: '',
  sms_code: '',
  new_password: '',
  confirm_password: ''
})
const resetCodeText = computed(() => {
  if (resetCountdown.value > 0) {
    return `${resetCountdown.value}s 后重发`
  }
  return '发送验证码'
})

function startCountdown(target, setTimer) {
  target.value = 60
  const timer = setInterval(() => {
    target.value--
    if (target.value <= 0) {
      clearInterval(timer)
    }
  }, 1000)
  setTimer(timer)
}

async function guideToRegister() {
  try {
    await showConfirmDialog({
      title: '手机号未注册',
      message: '该手机号尚未注册商家账号，是否前往申请入驻？',
      confirmButtonText: '去注册',
      cancelButtonText: '取消'
    })
    router.push('/m/merchant/register')
  } catch {
    // 用户取消，不做任何事。
  }
}

async function sendLoginCode() {
  if (sendingLoginCode.value || loginCountdown.value > 0) return
  const phone = form.value.phone
  if (!phone || !/^1[3-9]\d{9}$/.test(phone)) {
    showToast('请先输入正确的11位手机号')
    return
  }
  sendingLoginCode.value = true
  try {
    await sendMerchantLoginCode({ phone })
    showSuccessToast('验证码已发送（开发模式填 123456）')
    startCountdown(loginCountdown, (timer) => { loginTimer = timer })
  } catch (e) {
    const errMsg = e.response?.data?.message || '发送失败'
    if (errMsg === NOT_MERCHANT_MESSAGE) {
      await guideToRegister()
    } else {
      showFailToast(errMsg)
    }
  } finally {
    sendingLoginCode.value = false
  }
}

async function onLogin() {
  if (loginLoading.value) return
  loginLoading.value = true
  try {
    const payload = loginMode.value === 'code'
      ? { phone: form.value.phone, login_type: 'code', sms_code: form.value.code }
      : {
          phone: form.value.phone,
          password: form.value.password,
          login_type: 'password'
        }
    const res = await merchantLogin(payload)

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
    if (errMsg === UNREGISTERED_MESSAGE) {
      await guideToRegister()
    } else {
      showFailToast(errMsg)
    }
  } finally {
    loginLoading.value = false
  }
}

function openForgotPopup() {
  resetForm.value.phone = form.value.phone
  forgotVisible.value = true
}

async function sendResetCode() {
  if (sendingResetCode.value || resetCountdown.value > 0) return
  const phone = resetForm.value.phone
  if (!phone || !/^1[3-9]\d{9}$/.test(phone)) {
    showToast('请先输入正确的11位手机号')
    return
  }
  sendingResetCode.value = true
  try {
    await sendMerchantPasswordResetCode({ phone })
    showSuccessToast('验证码已发送（开发模式填 123456）')
    startCountdown(resetCountdown, (timer) => { resetTimer = timer })
  } catch (e) {
    showFailToast(e.response?.data?.message || '发送失败')
  } finally {
    sendingResetCode.value = false
  }
}

async function onResetPassword() {
  if (resetLoading.value) return
  resetLoading.value = true
  try {
    await resetMerchantPassword(resetForm.value)
    showSuccessToast('密码已重置，请使用新密码登录')
    forgotVisible.value = false
    form.value.phone = resetForm.value.phone
    loginMode.value = 'password'
  } catch (e) {
    showFailToast(e.response?.data?.message || '重置密码失败')
  } finally {
    resetLoading.value = false
  }
}

onUnmounted(() => {
  if (loginTimer) clearInterval(loginTimer)
  if (resetTimer) clearInterval(resetTimer)
})
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

.mode-switch {
  display: flex;
  justify-content: center;
  gap: 12px;
  padding: 0 16px 16px;
}

.form-submit {
  padding: 28px 16px 0;
}

.form-links {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 24px;
  padding: 0 16px;
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

.forgot-popup {
  padding: 20px 16px 32px;
}

.forgot-popup h3 {
  text-align: center;
  margin: 0 0 16px;
  font-size: 18px;
}

.forgot-submit {
  padding: 20px 16px 0;
}

.forgot-tip {
  margin: 16px;
  font-size: 12px;
  line-height: 1.6;
  color: #969799;
  text-align: center;
}
</style>
