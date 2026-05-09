<template>
  <div class="auth-grid">
    <div class="auth-panel page-card">
      <div class="eyebrow">{{ text.portalEyebrow }}</div>
      <h1>{{ text.loginTitle }}</h1>
      <p>{{ text.loginSubtitle }}</p>

      <el-segmented v-model="loginMode" :options="loginModeOptions" class="mode-switch" />

      <el-form v-if="loginMode === 'password'" :model="passwordLoginForm">
        <el-form-item>
          <el-input v-model="passwordLoginForm.account" :placeholder="text.accountPlaceholder" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="passwordLoginForm.password" type="password" show-password :placeholder="text.passwordPlaceholder" />
        </el-form-item>
        <el-button type="primary" style="width: 100%" @click="submitPasswordLogin">{{ text.passwordLoginButton }}</el-button>
      </el-form>

      <el-form v-else :model="phoneLoginForm">
        <el-form-item>
          <el-input v-model="phoneLoginForm.phone" maxlength="11" :placeholder="text.phonePlaceholder" />
        </el-form-item>
        <el-form-item class="code-row">
          <el-input v-model="phoneLoginForm.code" :placeholder="text.codePlaceholder" />
          <el-button
            class="code-button"
            :disabled="loginCountdown > 0"
            @click="sendLoginCode"
          >
            {{ loginCountdown > 0 ? `${text.resendPrefix} ${loginCountdown}s` : text.sendLoginCode }}
          </el-button>
        </el-form-item>
        <el-button type="primary" style="width: 100%" @click="submitPhoneLogin">{{ text.phoneLoginButton }}</el-button>
      </el-form>

      <el-button link type="primary" class="forgot-link" @click="router.push('/portal/forgot-password')">
        {{ text.forgotPassword }}
      </el-button>
    </div>

    <div class="auth-panel page-card">
      <div class="eyebrow">{{ text.registerEyebrow }}</div>
      <h1>{{ text.registerTitle }}</h1>
      <p>{{ text.registerSubtitle }}</p>
      <el-form :model="registerForm">
        <el-form-item>
          <el-input v-model="registerForm.display_name" :placeholder="text.displayNamePlaceholder" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="registerForm.phone" maxlength="11" :placeholder="text.phonePlaceholder" />
        </el-form-item>
        <el-form-item class="code-row">
          <el-input v-model="registerForm.sms_code" :placeholder="text.codePlaceholder" />
          <el-button
            class="code-button"
            :disabled="registerCountdown > 0"
            @click="sendRegisterCode"
          >
            {{ registerCountdown > 0 ? `${text.resendPrefix} ${registerCountdown}s` : text.sendRegisterCode }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-input v-model="registerForm.password" type="password" show-password :placeholder="text.passwordPlaceholder" />
        </el-form-item>
        <el-button type="success" style="width: 100%" @click="submitRegister">{{ text.registerButton }}</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { loginUser, loginUserByPhone, registerUser, sendLoginSMSCode, sendRegisterSMSCode } from '../../api/modules'
import { useUserAuthStore } from '../../stores/userAuth'

const text = {
  portalEyebrow: '\u7528\u6237\u4e2d\u5fc3',
  loginTitle: '\u767b\u5f55\u7528\u6237\u8d26\u53f7',
  loginSubtitle: '\u53ef\u4f7f\u7528\u663e\u793a\u540d\uff08\u5e97\u94fa\u6216\u4f01\u4e1a\u540d\u79f0\uff09+\u5bc6\u7801\u767b\u5f55\uff0c\u6216\u5207\u6362\u4e3a\u624b\u673a\u53f7\u9a8c\u8bc1\u7801\u767b\u5f55\u3002',
  registerEyebrow: '\u6ce8\u518c\u8d26\u53f7',
  registerTitle: '\u521b\u5efa\u65b0\u7528\u6237',
  registerSubtitle: '\u6ce8\u518c\u65f6\u9700\u8981 11 \u4f4d\u624b\u673a\u53f7\u3001\u77ed\u4fe1\u9a8c\u8bc1\u7801\u4ee5\u53ca\u552f\u4e00\u7684\u663e\u793a\u540d\uff08\u5e97\u94fa\u6216\u4f01\u4e1a\u540d\u79f0\uff09\u3002\u90ae\u7bb1\u5728\u4e2a\u4eba\u4e2d\u5fc3\u7ed1\u5b9a\u3002',
  accountPlaceholder: '\u663e\u793a\u540d\uff08\u5e97\u94fa\u6216\u4f01\u4e1a\u540d\u79f0\uff09',
  passwordPlaceholder: '\u5bc6\u7801',
  phonePlaceholder: '\u624b\u673a\u53f7\uff0811 \u4f4d\uff09',
  codePlaceholder: '\u8bf7\u8f93\u5165\u9a8c\u8bc1\u7801',
  displayNamePlaceholder: '\u663e\u793a\u540d\uff08\u5e97\u94fa\u6216\u4f01\u4e1a\u540d\u79f0\uff09',
  forgotPassword: '\u5fd8\u8bb0\u5bc6\u7801\uff1f',
  passwordMode: '\u8d26\u53f7\u5bc6\u7801',
  phoneMode: '\u624b\u673a\u9a8c\u8bc1\u7801',
  passwordLoginButton: '\u8d26\u53f7\u767b\u5f55',
  phoneLoginButton: '\u624b\u673a\u767b\u5f55',
  registerButton: '\u6ce8\u518c\u5e76\u767b\u5f55',
  sendLoginCode: '\u53d1\u9001\u767b\u5f55\u9a8c\u8bc1\u7801',
  sendRegisterCode: '\u53d1\u9001\u6ce8\u518c\u9a8c\u8bc1\u7801',
  resendPrefix: '\u91cd\u65b0\u53d1\u9001',
  loginCodeSent: '\u767b\u5f55\u9a8c\u8bc1\u7801\u5df2\u53d1\u9001\uff0c\u8bf7\u67e5\u770b\u540e\u7aef\u63a7\u5236\u53f0',
  registerCodeSent: '\u6ce8\u518c\u9a8c\u8bc1\u7801\u5df2\u53d1\u9001\uff0c\u8bf7\u67e5\u770b\u540e\u7aef\u63a7\u5236\u53f0',
  loginSuccess: '\u767b\u5f55\u6210\u529f',
  loginFailed: '\u767b\u5f55\u5931\u8d25',
  registerSuccess: '\u6ce8\u518c\u6210\u529f',
  registerFailed: '\u6ce8\u518c\u5931\u8d25',
  backendUnavailable: '\u540e\u7aef\u670d\u52a1\u672a\u542f\u52a8\uff0c\u8bf7\u5148\u542f\u52a8 Go \u670d\u52a1'
}

const router = useRouter()
const store = useUserAuthStore()
const loginMode = ref('password')
const loginCountdown = ref(0)
const registerCountdown = ref(0)
let loginTimer = null
let registerTimer = null

const loginModeOptions = computed(() => [
  { label: text.passwordMode, value: 'password' },
  { label: text.phoneMode, value: 'phone' }
])

const passwordLoginForm = reactive({
  account: '',
  password: ''
})

const phoneLoginForm = reactive({
  phone: '',
  code: ''
})

const registerForm = reactive({
  display_name: '',
  phone: '',
  sms_code: '',
  password: ''
})

const startCountdown = (target, saveTimer) => {
  target.value = 60
  const timer = window.setInterval(() => {
    target.value -= 1
    if (target.value <= 0) {
      window.clearInterval(timer)
      saveTimer(null)
    }
  }, 1000)
  saveTimer(timer)
}

const onAuthSuccess = (data) => {
  store.setAuth(data)
  router.push('/portal/packages')
}

const sendLoginCode = async () => {
  try {
    await sendLoginSMSCode({ account: phoneLoginForm.phone })
    ElMessage.success(text.loginCodeSent)
    startCountdown(loginCountdown, (timer) => {
      loginTimer = timer
    })
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.sendLoginCode)
  }
}

const sendRegisterCode = async () => {
  try {
    await sendRegisterSMSCode({ phone: registerForm.phone })
    ElMessage.success(text.registerCodeSent)
    startCountdown(registerCountdown, (timer) => {
      registerTimer = timer
    })
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.sendRegisterCode)
  }
}

const submitPasswordLogin = async () => {
  try {
    const res = await loginUser(passwordLoginForm)
    onAuthSuccess(res.data)
    ElMessage.success(text.loginSuccess)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.backendUnavailable)
  }
}

const submitPhoneLogin = async () => {
  try {
    const res = await loginUserByPhone(phoneLoginForm)
    onAuthSuccess(res.data)
    ElMessage.success(text.loginSuccess)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.backendUnavailable)
  }
}

const submitRegister = async () => {
  try {
    const res = await registerUser(registerForm)
    onAuthSuccess(res.data)
    ElMessage.success(text.registerSuccess)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.backendUnavailable)
  }
}

onBeforeUnmount(() => {
  if (loginTimer) {
    window.clearInterval(loginTimer)
  }
  if (registerTimer) {
    window.clearInterval(registerTimer)
  }
})
</script>

<style scoped>
.auth-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 20px;
}

.auth-panel h1 {
  margin: 0 0 8px;
  font-size: 32px;
}

.auth-panel p {
  color: var(--muted);
  line-height: 1.7;
  margin: 0 0 20px;
}

.mode-switch {
  margin-bottom: 18px;
}

.code-row :deep(.el-form-item__content) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 148px;
  gap: 10px;
  align-items: stretch;
}

.code-button {
  width: 148px;
  white-space: nowrap;
  margin-left: 0;
}

.code-button.is-disabled,
.code-button[disabled] {
  opacity: 1;
}

.forgot-link {
  margin-top: 12px;
  padding-left: 0;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.15em;
  margin-bottom: 8px;
}

@media (max-width: 520px) {
  .code-row :deep(.el-form-item__content) {
    grid-template-columns: 1fr;
  }

  .code-button {
    width: 100%;
  }
}
</style>
