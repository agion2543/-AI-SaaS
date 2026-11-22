<template>
  <div class="merchant-layout">
    <router-view />

    <!-- 底部导航栏 -->
    <van-tabbar
      v-model="activeTab"
      route
      fixed
      placeholder
      safe-area-inset-bottom
    >
      <van-tabbar-item to="/m/merchant/home" icon="home-o">
        首页
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/group-buy" icon="qr">
        团购
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/products" icon="shop-o">
        商品
      </van-tabbar-item>
      <van-tabbar-item to="/m/merchant/orders" icon="orders-o">
        订单
      </van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const activeTab = ref(0)

// 根据路由更新 TabBar
const tabRoutes = {
  '/m/merchant/home': 0,
  '/m/merchant/group-buy': 1,
  '/m/merchant/products': 2,
  '/m/merchant/orders': 3
}

watch(
  () => route.path,
  (path) => {
    if (tabRoutes[path] !== undefined) {
      activeTab.value = tabRoutes[path]
    } else if (path.startsWith('/m/merchant/orders/')) {
      activeTab.value = tabRoutes['/m/merchant/orders']
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.merchant-layout {
  min-height: 100vh;
  background: #f7f8fa;
}
</style>
