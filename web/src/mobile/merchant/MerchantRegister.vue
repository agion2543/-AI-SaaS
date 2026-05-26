<template>
  <div class="merchant-register">
    <!-- 背景装饰 -->
    <div class="register-bg">
      <div class="logo-area">
        <h1 class="app-name">商家入驻</h1>
        <p class="app-desc">开启您的数字化经营之旅</p>
      </div>
    </div>

    <!-- 注册表单 -->
    <div class="register-form">
      <van-form @submit="onSubmit">
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
            v-model="form.code"
            name="code"
            label="验证码"
            placeholder="请输入6位验证码"
            maxlength="6"
            type="digit"
            :rules="[
              { required: true, message: '请输入验证码' },
              { pattern: /^\d{6}$/, message: '验证码必须为6位数字' }
            ]"
          >
            <template #button>
              <van-button
                size="small"
                type="primary"
                :loading="sendingCode"
                :disabled="codeCountdown > 0"
                @click="sendCode"
              >
                {{ codeText }}
              </van-button>
            </template>
          </van-field>
          <van-field
            v-model="form.name"
            name="name"
            label="商家名称"
            placeholder="请输入商家名称"
            :rules="[{ required: true, message: '请输入商家名称' }]"
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
          <van-field
            v-model="form.confirmPassword"
            type="password"
            name="confirmPassword"
            label="确认密码"
            placeholder="请再次输入密码"
            :rules="[
              { required: true, message: '请确认密码' },
              {
                validator: (val) => val === form.password || '两次密码不一致'
              }
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
            开发模式：验证码固定为 <b>123456</b>，可直接使用
          </van-notice-bar>
        </div>

        <div class="form-submit">
          <van-button
            round
            block
            type="primary"
            native-type="submit"
            :loading="registerLoading"
          >
            注册
          </van-button>
        </div>

        <div class="agreement">
          注册即表示同意
          <a href="#" @click.prevent="showAgreement">《用户协议》</a>
          和
          <a href="#" @click.prevent="showPrivacy">《隐私政策》</a>
        </div>

        <div class="login-link">
          已有账号？
          <router-link to="/m/merchant/login">立即登录</router-link>
        </div>
      </van-form>
    </div>

    <!-- 协议弹窗 -->
    <van-popup v-model:show="showAgreementPopup" position="bottom" style="height: 60%;">
      <div class="agreement-content">
        <h3>用户协议</h3>
        <p>这里是用户协议内容...</p>
      </div>
    </van-popup>

    <van-popup v-model:show="showPrivacyPopup" position="bottom" style="height: 60%;">
      <div class="agreement-content">
        <h3>隐私政策</h3>
        <p>这里是隐私政策内容...</p>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import { merchantRegister, sendMerchantRegisterSMSCode } from '@/api/modules'

const router = useRouter()

const isDev = import.meta.env.DEV ?? true

const form = ref({
  phone: '',
  code: '',
  name: '',
  password: '',
  confirmPassword: ''
})

const registerLoading = ref(false)
const sendingCode = ref(false)
const codeCountdown = ref(0)
const showAgreementPopup = ref(false)
const showPrivacyPopup = ref(false)

const codeText = computed(() => {
  if (codeCountdown.value > 0) {
    return `${codeCountdown.value}s 后重新发送`
  }
  return '发送验证码'
})

async function sendCode() {
  const phone = form.value.phone
  if (!phone || !/^1[3-9]\d{9}$/.test(phone)) {
    showToast('请先输入正确的11位手机号')
    return
  }

  sendingCode.value = true
  try {
    await sendMerchantRegisterSMSCode({ phone })
    showSuccessToast('验证码已发送（开发模式填 123456）')

    codeCountdown.value = 60
    const timer = setInterval(() => {
      codeCountdown.value--
      if (codeCountdown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)
  } catch (e) {
    showFailToast(e.response?.data?.message || '发送失败')
  } finally {
    sendingCode.value = false
  }
}

async function onSubmit() {
  // 前端二次校验
  if (!/^1[3-9]\d{9}$/.test(form.value.phone)) {
    showToast('手机号必须为11位数字')
    return
  }
  if (!/^\d{6}$/.test(form.value.code)) {
    showToast('验证码必须为6位数字')
    return
  }
  if (form.value.password.length < 6) {
    showToast('密码至少6位')
    return
  }
  if (form.value.password !== form.value.confirmPassword) {
    showToast('两次密码不一致')
    return
  }

  registerLoading.value = true
  try {
    const res = await merchantRegister({
      contact_phone: form.value.phone,
      sms_code: form.value.code,
      name: form.value.name,
      password: form.value.password,
      confirm_password: form.value.confirmPassword
    })

    showSuccessToast('注册成功！正在跳转...')

    // 注册成功后自动保存 token 并跳转
    localStorage.setItem('merchant_token', res.data.token)
    localStorage.setItem('merchant_profile', JSON.stringify(res.data.merchant))

    router.replace('/m/merchant/home')
  } catch (e) {
    // 提取后端返回的具体错误信息
    const errMsg = e.response?.data?.message
      || e.message
      || '注册失败，请稍后重试'
    console.error('[注册失败]', e.response?.data || e.message || e)
    showFailToast(errMsg)
  } finally {
    registerLoading.value = false
  }
}

function showAgreement() {
  showAgreementPopup.value = true
}

function showPrivacy() {
  showPrivacyPopup.value = true
}
</script>

<style scoped>
.merchant-register {
  min-height: 100vh;
  background: #f7f8fa;
}

.register-bg {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 50px 20px 40px;
  color: white;
}

.logo-area {
  text-align: center;
}

.app-name {
  font-size: 28px;
  font-weight: bold;
  margin: 0 0 8px;
}

.app-desc {
  font-size: 14px;
  opacity: 0.9;
  margin: 0;
}

.register-form {
  background: white;
  border-radius: 16px 16px 0 0;
  padding: 24px 16px;
  margin-top: -20px;
  min-height: calc(100vh - 180px);
}

.form-submit {
  padding: 24px 16px 0;
}

.dev-notice {
  margin: 12px 16px;
}

.agreement {
  text-align: center;
  font-size: 12px;
  color: #999;
  margin-top: 16px;
  padding: 0 16px;
}

.agreement a {
  color: #667eea;
  text-decoration: none;
}

.login-link {
  text-align: center;
  margin-top: 20px;
  font-size: 14px;
  color: #666;
}

.login-link a {
  color: #667eea;
  text-decoration: none;
}

.agreement-content {
  padding: 24px;
}

.agreement-content h3 {
  text-align: center;
  margin-bottom: 16px;
}

.agreement-content p {
  font-size: 14px;
  line-height: 1.8;
  color: #666;
}
</style>
