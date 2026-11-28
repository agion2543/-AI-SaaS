<template>
  <main class="merchant-mine">
    <van-nav-bar title="我的" />

    <section class="mine-section" aria-label="账户与服务">
      <van-cell-group inset>
        <van-cell
          title="入驻审核"
          :value="onboardingValue"
          :label="onboardingLabel"
          is-link
          to="/m/merchant/subscription"
        />
        <van-cell
          title="账户与设置"
          label="商家资料、账号安全与其他设置"
          is-link
          to="/m/merchant/settings"
        />
        <van-cell
          title="订阅与业务状态"
          label="查看服务状态、续费与访问限制"
          is-link
          to="/m/merchant/subscription"
        />
      </van-cell-group>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchMerchantInfo } from '@/api/modules'
import { isMerchantOnboardingRejected, isMerchantPending } from '@/utils/merchantOnboarding'

const merchant = ref({})

const onboardingValue = computed(() => {
  if (isMerchantOnboardingRejected(merchant.value)) return '审核未通过'
  if (isMerchantPending(merchant.value)) return '待审核'
  return '已通过'
})

const onboardingLabel = computed(() => {
  if (isMerchantOnboardingRejected(merchant.value)) return `拒绝原因：${merchant.value.review_reason}`
  if (isMerchantPending(merchant.value)) return '通过后可开通订阅并营业'
  return '入驻审核已通过'
})

onMounted(async () => {
  try {
    const response = await fetchMerchantInfo()
    merchant.value = response?.data?.merchant || {}
  } catch {
    merchant.value = {}
  }
})
</script>

<style scoped>
.merchant-mine {
  min-height: 100vh;
  padding: 12px 0 76px;
  background: #f7f8fa;
}

.mine-section {
  margin-top: 12px;
}
</style>
