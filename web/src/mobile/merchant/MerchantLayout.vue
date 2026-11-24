<template>
  <div class="merchant-layout">
    <section v-if="maintenanceNotice" class="maintenance-notice" role="status">
      <div class="notice-heading">
        <van-icon name="info-o" />
        <strong>{{ maintenanceNotice.title }}</strong>
      </div>
      <p>维护窗口（北京时间）：{{ maintenanceNotice.window_start.replace('T', ' ') }} 至 {{ maintenanceNotice.window_end.replace('T', ' ') }}</p>
      <p><strong>可能影响：</strong>{{ maintenanceNotice.expected_impact }}</p>
      <p><strong>准备事项：</strong>{{ maintenanceNotice.preparation }}</p>
    </section>
    <router-view />

    <!-- 底部导航栏 -->
    <van-tabbar
      :model-value="activeTab"
      fixed
      placeholder
      safe-area-inset-bottom
    >
      <van-tabbar-item to="/m/merchant/home" icon="home-o">
        首页
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/orders" icon="orders-o">
        订单
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/products" icon="shop-o">
        商品
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/mine" icon="manager-o">
        我的
      </van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { fetchMerchantMaintenanceNotice } from '../../api/modules'
import { getMerchantMobileActiveTab } from '../../utils/navigation'

const route = useRoute()
const activeTab = computed(() => getMerchantMobileActiveTab(route.path))
const maintenanceNotice = ref(null)
let maintenanceNoticeTimer

const refreshMaintenanceNotice = async () => {
  try {
    const res = await fetchMerchantMaintenanceNotice()
    maintenanceNotice.value = res.data?.notice || null
  } catch {
    // 公告服务暂时不可用时不阻断商家操作，并保留已展示内容。
  }
}

onMounted(() => {
  refreshMaintenanceNotice()
  maintenanceNoticeTimer = window.setInterval(refreshMaintenanceNotice, 5 * 60 * 1000)
})

onBeforeUnmount(() => window.clearInterval(maintenanceNoticeTimer))
</script>

<style scoped>
.merchant-layout {
  min-height: 100vh;
  background: #f7f8fa;
}
.maintenance-notice {
  margin: 10px 12px 0;
  padding: 12px;
  border: 1px solid #f1d59b;
  border-radius: 12px;
  background: #fff8e8;
  color: #684d16;
  font-size: 12px;
  line-height: 1.55;
}
.notice-heading {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  margin-bottom: 5px;
  font-size: 14px;
}
.notice-heading :deep(.van-icon) {
  margin-top: 2px;
  flex: 0 0 auto;
}
.maintenance-notice p {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}
</style>
