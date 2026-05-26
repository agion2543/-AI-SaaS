<template>
  <div class="merchant-settings">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="设置"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <!-- 用户信息 -->
    <div class="user-section">
      <van-cell-group inset>
        <van-cell
          title="头像"
          is-link
          @click="changeAvatar"
        >
          <template #value>
            <van-image
              round
              width="48"
              height="48"
              :src="merchantInfo.avatar || defaultAvatar"
            />
          </template>
        </van-cell>
        <van-cell
          title="商家名称"
          :value="merchantInfo.name || '未设置'"
          is-link
          @click="showEditName = true"
        />
        <van-cell
          title="手机号"
          :value="merchantInfo.phone || '未绑定'"
          is-link
          @click="showEditPhone = true"
        />
      </van-cell-group>
    </div>

    <!-- 账号安全 -->
    <div class="section">
      <van-cell-group inset title="账号安全">
        <van-cell
          title="修改密码"
          is-link
          @click="showChangePassword = true"
        />
        <van-cell
          title="绑定邮箱"
          is-link
          :value="merchantInfo.email || '未绑定'"
          @click="showBindEmail = true"
        />
      </van-cell-group>
    </div>

    <!-- 支付设置 -->
    <div class="section">
      <van-cell-group inset title="支付设置">
        <van-cell
          title="微信支付配置"
          is-link
          @click="showPaymentConfig = true"
        />
        <van-cell
          title="支付二维码"
          is-link
          @click="showQRCode = true"
        />
      </van-cell-group>
    </div>

    <!-- 通知设置 -->
    <div class="section">
      <van-cell-group inset title="通知设置">
        <van-cell title="订单提醒">
          <template #value>
            <van-switch v-model="notifications.order" size="20" />
          </template>
        </van-cell>
        <van-cell title="退款提醒">
          <template #value>
            <van-switch v-model="notifications.refund" size="20" />
          </template>
        </van-cell>
        <van-cell title="系统通知">
          <template #value>
            <van-switch v-model="notifications.system" size="20" />
          </template>
        </van-cell>
      </van-cell-group>
    </div>

    <!-- 其他设置 -->
    <div class="section">
      <van-cell-group inset title="其他">
        <van-cell
          title="关于我们"
          is-link
          @click="showAbout = true"
        />
        <van-cell
          title="帮助与反馈"
          is-link
          @click="showHelp = true"
        />
        <van-cell
          title="清除缓存"
          is-link
          :value="cacheSize"
          @click="clearCache"
        />
      </van-cell-group>
    </div>

    <!-- 退出登录 -->
    <div class="logout-section">
      <van-button
        type="danger"
        plain
        block
        @click="logout"
      >
        退出登录
      </van-button>
    </div>

    <!-- 版本信息 -->
    <div class="version-info">
      <span>本地生活商家 AI 运营 SaaS v1.0.0</span>
    </div>

    <!-- 修改名称弹窗 -->
    <van-dialog
      v-model:show="showEditName"
      title="修改商家名称"
      show-cancel-button
      @confirm="saveName"
    >
      <van-field v-model="editName" placeholder="请输入商家名称" />
    </van-dialog>

    <!-- 修改手机号弹窗 -->
    <van-dialog
      v-model:show="showEditPhone"
      title="修改手机号"
      show-cancel-button
      @confirm="savePhone"
    >
      <div style="padding: 16px;">
        <van-field
          v-model="editPhone"
          placeholder="请输入新手机号"
          style="margin-bottom: 12px;"
        />
        <van-field
          v-model="editCode"
          center
          placeholder="请输入验证码"
        >
          <template #button>
            <van-button
              size="small"
              type="primary"
              :loading="sendingCode"
              @click="sendCode"
            >
              {{ codeText }}
            </van-button>
          </template>
        </van-field>
      </div>
    </van-dialog>

    <!-- 修改密码弹窗 -->
    <van-dialog
      v-model:show="showChangePassword"
      title="修改密码"
      show-cancel-button
      @confirm="changePassword"
    >
      <div style="padding: 16px;">
        <van-field
          v-model="passwordForm.oldPassword"
          type="password"
          label="原密码"
          placeholder="请输入原密码"
          style="margin-bottom: 12px;"
        />
        <van-field
          v-model="passwordForm.newPassword"
          type="password"
          label="新密码"
          placeholder="请输入新密码"
          style="margin-bottom: 12px;"
        />
        <van-field
          v-model="passwordForm.confirmPassword"
          type="password"
          label="确认密码"
          placeholder="请再次输入新密码"
        />
      </div>
    </van-dialog>

    <!-- 关于我们 -->
    <van-popup v-model:show="showAbout" position="bottom" style="height: 60%;">
      <div class="about-content">
        <div class="about-header">
          <van-image
            width="80"
            height="80"
            round
            :src="defaultAvatar"
          />
          <h3>本地生活商家 AI 运营 SaaS</h3>
          <p>版本 1.0.0</p>
        </div>
        <div class="about-desc">
          <p>本地生活商家 AI 运营 SaaS 平台，为本地生活商家提供智能化的运营管理解决方案。</p>
          <p>我们致力于帮助商家提升运营效率，降低经营成本，实现数字化转型升级。</p>
        </div>
        <div class="about-features">
          <h4>核心功能</h4>
          <ul>
            <li>智能点单系统</li>
            <li>订单管理</li>
            <li>营销工具</li>
            <li>AI 运营助手</li>
            <li>数据分析</li>
          </ul>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import { fetchMerchantInfo, updateMerchantInfo, changeMerchantPassword } from '@/api/modules'

const router = useRouter()

const merchantInfo = ref({})
const defaultAvatar = 'https://via.placeholder.com/48x48?text=商家'
const cacheSize = ref('2.5 MB')

const notifications = ref({
  order: true,
  refund: true,
  system: true
})

// 修改名称
const showEditName = ref(false)
const editName = ref('')

// 修改手机号
const showEditPhone = ref(false)
const editPhone = ref('')
const editCode = ref('')
const sendingCode = ref(false)
const codeCountdown = ref(0)

// 修改密码
const showChangePassword = ref(false)
const passwordForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

// 其他
const showAbout = ref(false)
const showHelp = ref(false)
const showPaymentConfig = ref(false)
const showQRCode = ref(false)
const showBindEmail = ref(false)

const codeText = computed(() => {
  if (codeCountdown.value > 0) {
    return `${codeCountdown.value}s`
  }
  return '发送验证码'
})

async function loadMerchantInfo() {
  try {
    const res = await fetchMerchantInfo()
    merchantInfo.value = res.data || {}
  } catch (e) {
    console.error('加载商家信息失败:', e)
  }
}

function changeAvatar() {
  showToast('头像上传功能开发中')
}

function saveName() {
  if (!editName.value.trim()) {
    showToast('请输入商家名称')
    return
  }
  doSaveName()
}

async function doSaveName() {
  try {
    await updateMerchantInfo({ name: editName.value })
    showSuccessToast('修改成功')
    merchantInfo.value.name = editName.value
  } catch (e) {
    showFailToast(e.response?.data?.message || '修改失败')
  }
}

function savePhone() {
  if (!editPhone.value || !/^1[3-9]\d{9}$/.test(editPhone.value)) {
    showToast('请输入正确的手机号')
    return
  }
  if (!editCode.value) {
    showToast('请输入验证码')
    return
  }
  doSavePhone()
}

async function doSavePhone() {
  try {
    await updateMerchantInfo({ phone: editPhone.value })
    showSuccessToast('修改成功')
    merchantInfo.value.phone = editPhone.value
    editPhone.value = ''
    editCode.value = ''
  } catch (e) {
    showFailToast(e.response?.data?.message || '修改失败')
  }
}

async function sendCode() {
  if (!editPhone.value || !/^1[3-9]\d{9}$/.test(editPhone.value)) {
    showToast('请输入正确的手机号')
    return
  }

  sendingCode.value = true
  try {
    await new Promise(resolve => setTimeout(resolve, 500))
    showSuccessToast('验证码已发送')
    codeCountdown.value = 60
    const timer = setInterval(() => {
      codeCountdown.value--
      if (codeCountdown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)
  } catch (e) {
    showFailToast('发送失败')
  } finally {
    sendingCode.value = false
  }
}

function changePassword() {
  if (!passwordForm.value.oldPassword) {
    showToast('请输入原密码')
    return
  }
  if (!passwordForm.value.newPassword) {
    showToast('请输入新密码')
    return
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    showToast('两次输入的密码不一致')
    return
  }
  doChangePassword()
}

async function doChangePassword() {
  try {
    await changeMerchantPassword({
      old_password: passwordForm.value.oldPassword,
      new_password: passwordForm.value.newPassword
    })
    showSuccessToast('密码修改成功')
    passwordForm.value = {
      oldPassword: '',
      newPassword: '',
      confirmPassword: ''
    }
  } catch (e) {
    showFailToast(e.response?.data?.message || '修改失败')
  }
}

function clearCache() {
  showDialog({
    title: '清除缓存',
    message: '确定要清除缓存吗？',
    showCancelButton: true,
  }).then(() => {
    // 模拟清除缓存
    cacheSize.value = '0 KB'
    showSuccessToast('缓存已清除')
  }).catch(() => {})
}

function logout() {
  showDialog({
    title: '退出登录',
    message: '确定要退出登录吗？',
    showCancelButton: true,
  }).then(() => {
    localStorage.removeItem('merchant_token')
    localStorage.removeItem('merchant_profile')
    localStorage.removeItem('merchant_current')
    router.replace('/m/merchant/login')
  }).catch(() => {})
}

onMounted(() => {
  loadMerchantInfo()
})
</script>

<style scoped>
.merchant-settings {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 40px;
}

.user-section {
  margin-top: 12px;
}

.section {
  margin-top: 12px;
}

.logout-section {
  margin: 24px 16px;
}

.version-info {
  text-align: center;
  color: #999;
  font-size: 12px;
  padding: 16px;
}

.about-content {
  padding: 24px;
}

.about-header {
  text-align: center;
  margin-bottom: 24px;
}

.about-header h3 {
  margin: 16px 0 8px;
  font-size: 18px;
}

.about-header p {
  color: #999;
  margin: 0;
}

.about-desc {
  margin-bottom: 24px;
}

.about-desc p {
  font-size: 14px;
  line-height: 1.8;
  color: #666;
  margin-bottom: 12px;
}

.about-features h4 {
  margin-bottom: 12px;
}

.about-features ul {
  padding-left: 20px;
  color: #666;
}

.about-features li {
  margin-bottom: 8px;
}
</style>
