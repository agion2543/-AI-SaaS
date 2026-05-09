<template>
  <div class="reset-shell page-card">
    <div class="eyebrow">{{ text.eyebrow }}</div>
    <h1>{{ text.title }}</h1>
    <p>{{ text.subtitle }}</p>

    <el-form :model="form">
      <el-form-item>
        <el-input v-model="form.phone" :placeholder="text.phonePlaceholder" />
      </el-form-item>
      <el-form-item class="code-row">
        <el-input v-model="form.code" :placeholder="text.codePlaceholder" />
        <el-button :disabled="countdown > 0" @click="sendCode">
          {{ countdown > 0 ? `${countdown}s` : text.sendCode }}
        </el-button>
      </el-form-item>
      <el-form-item>
        <el-input v-model="form.new_password" type="password" show-password :placeholder="text.passwordPlaceholder" />
      </el-form-item>
      <el-form-item>
        <el-input v-model="form.confirm_password" type="password" show-password :placeholder="text.confirmPasswordPlaceholder" />
      </el-form-item>
      <el-button type="primary" style="width: 100%" @click="submit">{{ text.submit }}</el-button>
      <el-button link type="primary" class="back-link" @click="router.push('/portal/login')">
        {{ text.backToLogin }}
      </el-button>
    </el-form>
  </div>
</template>

<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resetPasswordByPhone, sendPasswordResetCode } from '../../api/modules'

const text = {
  eyebrow: '\u627e\u56de\u5bc6\u7801',
  title: '\u901a\u8fc7\u624b\u673a\u53f7\u91cd\u7f6e\u5bc6\u7801',
  subtitle: '\u5148\u53d1\u9001\u9a8c\u8bc1\u7801\uff0c\u7136\u540e\u9a8c\u8bc1\u624b\u673a\u53f7\u5e76\u8bbe\u7f6e\u65b0\u5bc6\u7801\u3002\u5f53\u524d\u4e3a\u6a21\u62df\u77ed\u4fe1\uff0c\u9a8c\u8bc1\u7801\u4f1a\u6253\u5370\u5728\u540e\u7aef\u63a7\u5236\u53f0\u3002',
  phonePlaceholder: '\u8bf7\u8f93\u5165\u624b\u673a\u53f7',
  codePlaceholder: '\u8bf7\u8f93\u5165\u77ed\u4fe1\u9a8c\u8bc1\u7801',
  passwordPlaceholder: '\u8bf7\u8f93\u5165\u65b0\u5bc6\u7801',
  confirmPasswordPlaceholder: '\u8bf7\u518d\u6b21\u8f93\u5165\u65b0\u5bc6\u7801',
  sendCode: '\u53d1\u9001\u9a8c\u8bc1\u7801',
  submit: '\u91cd\u7f6e\u5bc6\u7801',
  backToLogin: '\u8fd4\u56de\u767b\u5f55',
  codeSent: '\u9a8c\u8bc1\u7801\u5df2\u53d1\u9001\uff0c\u8bf7\u67e5\u770b\u540e\u7aef\u63a7\u5236\u53f0',
  resetSuccess: '\u5bc6\u7801\u5df2\u91cd\u7f6e\uff0c\u8bf7\u91cd\u65b0\u767b\u5f55',
  passwordMismatch: '\u4e24\u6b21\u8f93\u5165\u7684\u5bc6\u7801\u4e0d\u4e00\u81f4'
}

const router = useRouter()
const form = reactive({
  phone: '',
  code: '',
  new_password: '',
  confirm_password: ''
})

const countdown = ref(0)
let timer = null

const startCountdown = () => {
  countdown.value = 60
  timer = window.setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) {
      window.clearInterval(timer)
      timer = null
    }
  }, 1000)
}

const sendCode = async () => {
  try {
    await sendPasswordResetCode({ phone: form.phone })
    ElMessage.success(text.codeSent)
    startCountdown()
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.sendCode)
  }
}

const submit = async () => {
  if (form.new_password !== form.confirm_password) {
    ElMessage.error(text.passwordMismatch)
    return
  }

  try {
    await resetPasswordByPhone({
      phone: form.phone,
      code: form.code,
      new_password: form.new_password
    })
    ElMessage.success(text.resetSuccess)
    router.push('/portal/login')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.submit)
  }
}

onBeforeUnmount(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})
</script>

<style scoped>
.reset-shell {
  max-width: 560px;
  margin: 0 auto;
}

.reset-shell h1 {
  margin: 0 0 8px;
}

.reset-shell p {
  color: var(--muted);
  line-height: 1.7;
  margin: 0 0 20px;
}

.code-row :deep(.el-form-item__content) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
}

.back-link {
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
</style>
