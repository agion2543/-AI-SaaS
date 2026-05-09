<template>
  <div class="register-shell">
    <section class="page-card register-card">
      <div class="eyebrow">MERCHANT APPLY</div>
      <h1>注册商家账号</h1>
      <p>注册后即可登录并进入订阅付费流程，完成订阅后解锁商家工作台。</p>

      <el-form label-position="top">
        <el-form-item label="商家名称">
          <el-input v-model="form.name" placeholder="请输入店铺或企业名称" maxlength="80" />
        </el-form-item>
        <el-form-item label="联系人手机号">
          <el-input v-model="form.contact_phone" placeholder="请输入 11 位手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="短信验证码">
          <div class="code-row">
            <el-input v-model="form.sms_code" placeholder="开发环境可输入 123456" maxlength="6" />
            <el-button :disabled="codeCountdown > 0 || sendingCode" :loading="sendingCode" @click="sendCode">
              {{ codeCountdown > 0 ? `${codeCountdown}s 后重发` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="form.confirm_password" type="password" show-password placeholder="再次输入密码" />
        </el-form-item>
        <el-button type="primary" size="large" :loading="submitting" class="full-button" @click="submit">
          注册账号
        </el-button>
        <el-button text class="full-button login-link" @click="router.push('/merchant/login')">
          已有账号，返回登录
        </el-button>
      </el-form>
    </section>
  </div>
</template>

<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { merchantRegister, sendMerchantRegisterSMSCode } from '../../api/modules'

const router = useRouter()
const submitting = ref(false)
const sendingCode = ref(false)
const codeCountdown = ref(0)
let timer = null

const form = reactive({
  name: '',
  contact_phone: '',
  sms_code: '',
  password: '',
  confirm_password: ''
})

const validPhone = () => /^1\d{10}$/.test(form.contact_phone.trim())

const startCountdown = () => {
  codeCountdown.value = 60
  timer = window.setInterval(() => {
    codeCountdown.value -= 1
    if (codeCountdown.value <= 0) {
      window.clearInterval(timer)
      timer = null
    }
  }, 1000)
}

const sendCode = async () => {
  if (!validPhone()) {
    ElMessage.warning('请输入 11 位有效手机号')
    return
  }
  sendingCode.value = true
  try {
    await sendMerchantRegisterSMSCode({ phone: form.contact_phone.trim() })
    ElMessage.success('验证码已发送，开发环境固定为 123456')
    startCountdown()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '验证码发送失败')
  } finally {
    sendingCode.value = false
  }
}

const submit = async () => {
  if (!form.name.trim()) {
    ElMessage.warning('请输入商家名称')
    return
  }
  if (!validPhone()) {
    ElMessage.warning('请输入 11 位有效手机号')
    return
  }
  if (!/^\d{6}$/.test(form.sms_code.trim())) {
    ElMessage.warning('请输入 6 位短信验证码')
    return
  }
  if (form.password.length < 6) {
    ElMessage.warning('密码至少 6 位')
    return
  }
  if (form.password !== form.confirm_password) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }

  submitting.value = true
  try {
    await merchantRegister({
      name: form.name.trim(),
      contact_phone: form.contact_phone.trim(),
      sms_code: form.sms_code.trim(),
      password: form.password,
      confirm_password: form.confirm_password
    })
    await ElMessageBox.alert('注册成功，请登录并完成订阅', '注册成功', {
      confirmButtonText: '去登录',
      type: 'success'
    })
    router.push('/merchant/login')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '注册失败')
  } finally {
    submitting.value = false
  }
}

onBeforeUnmount(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})
</script>

<style scoped>
.register-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.18), transparent 26%),
    linear-gradient(145deg, #f7fbff 0%, #ebf4fb 100%);
}

.register-card {
  width: min(520px, 100%);
  padding: 30px;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

h1 {
  margin: 8px 0;
  font-size: 34px;
}

p {
  margin: 0 0 22px;
  color: var(--muted);
  line-height: 1.7;
}

.code-row {
  display: grid;
  grid-template-columns: 1fr 128px;
  gap: 10px;
  width: 100%;
}

.full-button {
  width: 100%;
}

.login-link {
  margin-top: 10px;
}

@media (max-width: 560px) {
  .code-row {
    grid-template-columns: 1fr;
  }
}
</style>
