<template>
  <div class="login-shell">
    <div class="login-panel">
      <div class="eyebrow">ADMIN CONSOLE</div>
      <h1>{{ text.title }}</h1>
      <p>{{ text.description }}</p>
      <el-form :model="form" @submit.prevent="submit">
        <el-form-item>
          <el-input v-model="form.username" :placeholder="text.username" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" :placeholder="text.password" show-password />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" @click="submit">{{ text.login }}</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminLogin } from '../../api/modules'
import { useAuthStore } from '../../stores/auth'

const text = {
  title: '\u5546\u4e1a\u5316 SaaS \u7ba1\u7406\u540e\u53f0',
  description: '\u7edf\u4e00\u7ba1\u7406\u7528\u6237\u3001\u5546\u5bb6\u3001\u5957\u9910\u3001\u8ba2\u5355\u3001\u652f\u4ed8\u8bb0\u5f55\u3001\u5361\u5bc6\u4e0e\u7cfb\u7edf\u914d\u7f6e\u3002',
  username: '\u7ba1\u7406\u5458\u8d26\u53f7',
  password: '\u767b\u5f55\u5bc6\u7801',
  login: '\u767b\u5f55\u540e\u53f0',
  success: '\u767b\u5f55\u6210\u529f',
  failed: '\u540e\u7aef\u670d\u52a1\u672a\u542f\u52a8\uff0c\u8bf7\u5148\u542f\u52a8 Go \u670d\u52a1'
}

const router = useRouter()
const store = useAuthStore()
const form = reactive({
  username: 'admin',
  password: 'Admin@123456'
})

const submit = async () => {
  try {
    const res = await adminLogin(form)
    store.setAuth(res.data)
    ElMessage.success(text.success)
    router.push('/admin/dashboard')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.failed)
  }
}
</script>

<style scoped>
.login-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background:
    radial-gradient(circle at top right, rgba(31, 111, 235, 0.25), transparent 28%),
    linear-gradient(145deg, #f7fbff 0%, #e9eef7 100%);
}

.login-panel {
  width: min(460px, 92vw);
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(18px);
  padding: 36px;
  border-radius: 8px;
  box-shadow: 0 22px 80px rgba(15, 23, 42, 0.12);
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.16em;
  margin-bottom: 10px;
}

h1 {
  margin: 0;
  font-size: 32px;
}

p {
  color: var(--muted);
  line-height: 1.7;
  margin: 12px 0 24px;
}
</style>
