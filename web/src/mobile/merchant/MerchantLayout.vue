<template>
  <div class="merchant-layout">
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
      <van-tabbar-item to="/m/merchant/finance" icon="balance-o">
        财务
      </van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getMerchantMobileActiveTab } from '../../utils/navigation'

const route = useRoute()
const activeTab = computed(() => getMerchantMobileActiveTab(route.path))
</script>

<style scoped>
.merchant-layout {
  min-height: 100vh;
  background: #f7f8fa;
}
</style>
