<template>
  <div class="profile-grid">
    <div class="page-card">
      <h2 class="page-title">{{ text.profileTitle }}</h2>
      <div class="detail-row"><span>{{ text.displayName }}</span><strong>{{ profile.display_name || '-' }}</strong></div>
      <div class="detail-row"><span>{{ text.email }}</span><strong>{{ profile.email || text.notBound }}</strong></div>
      <div class="detail-row"><span>{{ text.phone }}</span><strong>{{ profile.phone || text.notBound }}</strong></div>
      <div class="detail-row"><span>{{ text.memberLevel }}</span><strong>{{ profile.member_level || 'free' }}</strong></div>
      <div class="detail-row"><span>{{ text.expiredAt }}</span><strong>{{ profile.expired_at || text.notActivated }}</strong></div>
      <div class="detail-row"><span>{{ text.remainingQuota }}</span><strong>{{ profile.remaining_quota ?? 0 }}</strong></div>
      <div class="detail-row"><span>{{ text.boundDevices }}</span><strong>{{ profile.bound_devices || text.none }}</strong></div>
    </div>

    <div class="page-card">
      <h2 class="page-title">{{ text.bindEmailTitle }}</h2>
      <p class="panel-copy">{{ text.bindEmailSubtitle }}</p>
      <el-form :model="emailForm">
        <el-form-item>
          <el-input v-model="emailForm.email" :placeholder="text.emailPlaceholder" />
        </el-form-item>
        <el-form-item class="code-row">
          <el-input v-model="emailForm.code" :placeholder="text.codePlaceholder" />
          <el-button :disabled="countdown > 0" @click="sendEmailCode">
            {{ countdown > 0 ? `${countdown}s` : text.sendCode }}
          </el-button>
        </el-form-item>
        <el-button type="primary" @click="submitBindEmail">{{ text.bindEmailButton }}</el-button>
      </el-form>
    </div>

    <div class="page-card usage-card">
      <h2 class="page-title">{{ text.usageTitle }}</h2>
      <el-table :data="usageRecords">
        <el-table-column prop="scene" :label="text.sceneColumn" />
        <el-table-column prop="description" :label="text.descriptionColumn" />
        <el-table-column prop="device_id" :label="text.deviceColumn" />
      </el-table>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { bindUserEmail, fetchUserProfile, sendBindEmailCode } from '../../api/modules'
import { useUserAuthStore } from '../../stores/userAuth'

const text = {
  profileTitle: '\u4e2a\u4eba\u4fe1\u606f',
  displayName: '\u6635\u79f0',
  email: '\u90ae\u7bb1',
  phone: '\u624b\u673a\u53f7',
  memberLevel: '\u4f1a\u5458\u7b49\u7ea7',
  expiredAt: '\u5230\u671f\u65f6\u95f4',
  remainingQuota: '\u5269\u4f59\u989d\u5ea6',
  boundDevices: '\u7ed1\u5b9a\u8bbe\u5907',
  usageTitle: '\u4f7f\u7528\u8bb0\u5f55',
  sceneColumn: '\u573a\u666f',
  descriptionColumn: '\u8bf4\u660e',
  deviceColumn: '\u8bbe\u5907',
  bindEmailTitle: '\u7ed1\u5b9a\u90ae\u7bb1',
  bindEmailSubtitle: '\u8f93\u5165\u771f\u5b9e\u53ef\u7528\u7684\u90ae\u7bb1\u5730\u5740\uff0c\u53d1\u9001\u9a8c\u8bc1\u7801\u540e\u518d\u5b8c\u6210\u7ed1\u5b9a\u3002\u5f53\u524d\u4e3a\u6a21\u62df\u90ae\u4ef6\uff0c\u9a8c\u8bc1\u7801\u4f1a\u6253\u5370\u5728\u540e\u7aef\u63a7\u5236\u53f0\u3002',
  emailPlaceholder: '\u8bf7\u8f93\u5165\u6709\u6548\u90ae\u7bb1',
  codePlaceholder: '\u8bf7\u8f93\u5165\u90ae\u7bb1\u9a8c\u8bc1\u7801',
  sendCode: '\u53d1\u9001\u90ae\u7bb1\u9a8c\u8bc1\u7801',
  bindEmailButton: '\u4fdd\u5b58\u90ae\u7bb1',
  notBound: '\u672a\u7ed1\u5b9a',
  notActivated: '\u672a\u5f00\u901a',
  none: '\u6682\u65e0',
  emailSent: '\u90ae\u7bb1\u9a8c\u8bc1\u7801\u5df2\u53d1\u9001\uff0c\u8bf7\u67e5\u770b\u540e\u7aef\u63a7\u5236\u53f0',
  emailBound: '\u90ae\u7bb1\u7ed1\u5b9a\u6210\u529f'
}

const store = useUserAuthStore()
const profile = ref({})
const usageRecords = ref([])
const countdown = ref(0)
const emailForm = reactive({
  email: '',
  code: ''
})
let timer = null

const loadProfile = async () => {
  const res = await fetchUserProfile()
  profile.value = res.data.profile
  usageRecords.value = res.data.usage_records || []
  emailForm.email = res.data.profile.email || ''
  store.setProfile(res.data.profile)
}

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

const sendEmailCode = async () => {
  try {
    await sendBindEmailCode({ email: emailForm.email })
    ElMessage.success(text.emailSent)
    startCountdown()
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.sendCode)
  }
}

const submitBindEmail = async () => {
  try {
    const res = await bindUserEmail(emailForm)
    profile.value = res.data.profile
    store.setProfile(res.data.profile)
    ElMessage.success(text.emailBound)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.bindEmailButton)
  }
}

onMounted(loadProfile)

onBeforeUnmount(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})
</script>

<style scoped>
.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.usage-card {
  grid-column: 1 / -1;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #eef2f7;
}

.panel-copy {
  margin: 0 0 16px;
  color: var(--muted);
  line-height: 1.7;
}

.code-row :deep(.el-form-item__content) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
}

@media (max-width: 900px) {
  .profile-grid {
    grid-template-columns: 1fr;
  }

  .usage-card {
    grid-column: auto;
  }
}
</style>
