<template>
  <div class="merchant-profile-grid">
    <div class="page-card">
      <h2 class="page-title">{{ text.title }}</h2>
      <el-alert
        v-if="merchant.status === 'pending'"
        :title="text.pendingNotice"
        type="warning"
        :closable="false"
        class="status-alert"
      />

      <div class="detail-row"><span>{{ text.name }}</span><strong>{{ merchant.name || '-' }}</strong></div>
      <div class="detail-row"><span>{{ text.phone }}</span><strong>{{ merchant.contact_phone || '-' }}</strong></div>
      <div class="detail-row"><span>{{ text.email }}</span><strong>{{ merchant.contact_email || '-' }}</strong></div>
      <div class="detail-row"><span>{{ text.status }}</span><strong>{{ statusLabel(merchant.status) }}</strong></div>
      <div class="detail-row"><span>{{ text.createdAt }}</span><strong>{{ merchant.created_at || '-' }}</strong></div>
    </div>

    <div class="page-card">
      <h2 class="page-title">{{ text.editTitle }}</h2>
      <el-form :model="form" label-position="top">
        <el-form-item :label="text.name">
          <el-input v-model="form.name" :placeholder="text.namePlaceholder" />
        </el-form-item>
        <el-form-item :label="text.phone">
          <el-input v-model="form.contact_phone" maxlength="11" :placeholder="text.phonePlaceholder" />
        </el-form-item>
        <el-form-item :label="text.email">
          <el-input v-model="form.contact_email" :placeholder="text.emailPlaceholder" />
        </el-form-item>
        <el-button type="primary" @click="submit">{{ text.save }}</el-button>
      </el-form>
    </div>

    <div class="page-card">
      <h2 class="page-title">修改登录密码</h2>
      <p class="muted">建议定期修改密码。忘记密码时，可在商家登录页通过手机号验证码重置。</p>
      <el-form :model="passwordForm" label-position="top">
        <el-form-item label="原密码">
          <el-input v-model="passwordForm.old_password" type="password" show-password placeholder="请输入当前密码" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="passwordForm.new_password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="passwordForm.confirm_password" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
        <el-button type="primary" @click="submitPassword">保存新密码</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { changeMerchantPassword, fetchMerchantInfo, updateMerchantInfo } from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

const text = {
  title: '\u6211\u7684\u5546\u5bb6',
  pendingNotice: '\u60a8\u7684\u8d26\u53f7\u6b63\u5728\u5ba1\u6838\u4e2d\uff0c\u8bf7\u8010\u5fc3\u7b49\u5f85',
  name: '\u5546\u5bb6\u540d\u79f0',
  phone: '\u8054\u7cfb\u7535\u8bdd',
  email: '\u8054\u7cfb\u90ae\u7bb1',
  status: '\u72b6\u6001',
  createdAt: '\u521b\u5efa\u65f6\u95f4',
  editTitle: '\u7f16\u8f91\u5546\u5bb6\u4fe1\u606f',
  namePlaceholder: '\u8bf7\u8f93\u5165\u5546\u5bb6\u540d\u79f0',
  phonePlaceholder: '\u8bf7\u8f93\u5165\u8054\u7cfb\u7535\u8bdd',
  emailPlaceholder: '\u8bf7\u8f93\u5165\u8054\u7cfb\u90ae\u7bb1',
  save: '\u4fdd\u5b58\u4fee\u6539',
  saved: '\u5546\u5bb6\u4fe1\u606f\u5df2\u66f4\u65b0',
  failed: '\u4fdd\u5b58\u5931\u8d25',
  pending: '\u5f85\u5ba1\u6838',
  active: '\u5df2\u542f\u7528',
  suspended: '\u5df2\u7981\u7528'
}

const authStore = useMerchantAuthStore()
const merchant = ref({})
const form = reactive({
  name: '',
  contact_phone: '',
  contact_email: ''
})
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const statusLabel = (status) => ({
  pending: text.pending,
  active: text.active,
  suspended: text.suspended
}[status] || status || '-')

const syncForm = (data) => {
  merchant.value = data
  form.name = data.name || ''
  form.contact_phone = data.contact_phone || ''
  form.contact_email = data.contact_email || ''
}

const load = async () => {
  const res = await fetchMerchantInfo()
  syncForm(res.data.merchant)
  authStore.merchant = res.data.merchant
  localStorage.setItem('merchant_current', JSON.stringify(res.data.merchant))
}

const submit = async () => {
  try {
    const res = await updateMerchantInfo(form)
    syncForm(res.data.merchant)
    authStore.merchant = res.data.merchant
    localStorage.setItem('merchant_current', JSON.stringify(res.data.merchant))
    ElMessage.success(text.saved)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.failed)
  }
}

const submitPassword = async () => {
  try {
    await changeMerchantPassword(passwordForm)
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
    ElMessage.success('登录密码已修改')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '密码修改失败')
  }
}

onMounted(load)
</script>

<style scoped>
.merchant-profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.status-alert {
  margin-bottom: 16px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #eef2f7;
}

.muted {
  color: #64748b;
  line-height: 1.7;
  margin: 0 0 16px;
}

@media (max-width: 960px) {
  .merchant-profile-grid {
    grid-template-columns: 1fr;
  }
}
</style>
