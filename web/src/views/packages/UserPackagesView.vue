<template>
  <div class="packages-shell">
    <div class="hero page-card">
      <div>
        <div class="eyebrow">{{ text.eyebrow }}</div>
        <h1>{{ text.title }}</h1>
        <p>{{ text.subtitle }}</p>
      </div>
      <el-button v-if="!userStore.token" type="primary" @click="router.push('/portal/login')">
        {{ text.signInFirst }}
      </el-button>
    </div>

    <div class="package-grid">
      <div v-for="item in packages" :key="item.id" class="package-card">
        <div class="package-tag">{{ item.code.toUpperCase() }}</div>
        <h3>{{ item.name }}</h3>
        <div class="price">{{ formatPrice(item.price) }}</div>
        <div class="meta">{{ formatDuration(item) }}</div>
        <div class="meta">{{ formatQuota(item.quota) }}</div>
        <p class="description">{{ item.description }}</p>
        <div class="actions">
          <el-button type="primary" :disabled="!userStore.token" @click="checkout(item, 'page')">
            {{ text.alipayPay }}
          </el-button>
          <el-button :disabled="!userStore.token" @click="checkout(item, 'qr')">
            {{ text.qrPay }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createUserOrder, fetchPublicPackages } from '../../api/modules'
import { useUserAuthStore } from '../../stores/userAuth'

const text = {
  eyebrow: '\u4f1a\u5458\u8d2d\u4e70',
  title: '\u9009\u62e9\u9002\u5408\u4f60\u7684\u4f1a\u5458\u5957\u9910',
  subtitle: '\u652f\u6301\u652f\u4ed8\u5b9d\u8df3\u8f6c\u652f\u4ed8\u548c\u626b\u7801\u652f\u4ed8\u3002\u521b\u5efa\u8ba2\u5355\u540e\u4f1a\u81ea\u52a8\u8fdb\u5165\u652f\u4ed8\u9875\uff0c\u5e76\u8f6e\u8be2\u8ba2\u5355\u652f\u4ed8\u72b6\u6001\u3002',
  signInFirst: '\u5148\u767b\u5f55\u518d\u8d2d\u4e70',
  alipayPay: '\u652f\u4ed8\u5b9d\u652f\u4ed8',
  qrPay: '\u626b\u7801\u652f\u4ed8',
  durationPrefix: '\u65f6\u957f\uff1a',
  durationLifetime: '\u6c38\u4e45',
  durationDays: '\u5929',
  quotaPrefix: '\u989d\u5ea6\uff1a',
  createOrderFailed: '\u521b\u5efa\u8ba2\u5355\u5931\u8d25',
  loadFailed: '\u5957\u9910\u52a0\u8f7d\u5931\u8d25\uff0c\u8bf7\u786e\u8ba4\u540e\u7aef\u670d\u52a1\u5df2\u542f\u52a8'
}

const router = useRouter()
const userStore = useUserAuthStore()
const packages = ref([])
const PAYMENT_CACHE_KEY = 'user_payment_cache'

const formatPrice = (price) => `\uffe5 ${(price / 100).toFixed(2)}`
const formatDuration = (item) => {
  if (item.is_lifetime) {
    return `${text.durationPrefix}${text.durationLifetime}`
  }
  return `${text.durationPrefix}${item.duration_days} ${text.durationDays}`
}
const formatQuota = (quota) => `${text.quotaPrefix}${quota}`

const load = async () => {
  try {
    const res = await fetchPublicPackages()
    packages.value = res.data
  } catch (error) {
    packages.value = []
    ElMessage.error(error.response?.data?.message || text.loadFailed)
  }
}

const checkout = async (pkg, payMode) => {
  try {
    const res = await createUserOrder({
      package_id: pkg.id,
      payment_channel: 'alipay',
      pay_mode: payMode,
      auto_renew: false
    })

    const order = res.data.order
    const payment = res.data.payment
    const cache = JSON.parse(window.sessionStorage.getItem(PAYMENT_CACHE_KEY) || '{}')
    cache[order.order_no] = payment
    window.sessionStorage.setItem(PAYMENT_CACHE_KEY, JSON.stringify(cache))

    router.push({
      path: '/portal/pay',
      query: {
        orderNo: order.order_no,
        mode: payment.mode,
        paymentUrl: payment.payment_url || '',
        qrCode: payment.qr_code || ''
      }
    })
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.createOrderFailed)
  }
}

onMounted(load)
</script>

<style scoped>
.packages-shell {
  display: grid;
  gap: 20px;
}

.hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}

.hero h1 {
  margin: 0 0 8px;
  font-size: 34px;
}

.hero p {
  margin: 0;
  color: var(--muted);
}

.package-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 18px;
}

.package-card {
  min-width: 0;
  background: linear-gradient(180deg, #fff 0%, #f4f8ff 100%);
  border: 1px solid #e5edf9;
  border-radius: 22px;
  padding: 24px;
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.06);
}

.package-card h3 {
  margin: 14px 0 6px;
}

.package-tag {
  display: inline-flex;
  padding: 6px 10px;
  border-radius: 999px;
  background: var(--brand-soft);
  color: var(--brand);
  font-size: 12px;
  font-weight: 700;
}

.price {
  font-size: 30px;
  font-weight: 800;
  margin: 12px 0 8px;
}

.meta {
  color: var(--muted);
  margin-bottom: 6px;
}

.description {
  min-height: 44px;
}

.actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.actions :deep(.el-button) {
  margin-left: 0;
  min-width: 0;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.15em;
  margin-bottom: 8px;
}

@media (min-width: 768px) {
  .package-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1200px) {
  .package-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 767px) {
  .hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
