<template>
  <div class="merchant-announcements">
    <section v-if="activePlan" class="maintenance-banner" role="status">
      <strong>平台维护中</strong>
      <span>{{ maintenanceBlockText(activePlan) }}</span>
    </section>

    <section class="page-card hero">
      <div>
        <div class="eyebrow">ANNOUNCEMENTS</div>
        <h1>平台公告</h1>
        <p>系统更新、活动促销与价格变动都在这里，发布时间与生效期以平台公告为准。</p>
      </div>
      <el-button type="primary" @click="loadAll">刷新</el-button>
    </section>

    <section class="page-card">
      <div v-if="loading" class="empty-hint">加载中…</div>
      <div v-else-if="!rows.length" class="empty-hint">暂无公告。</div>
      <article v-for="row in rows" :key="row.id" class="announcement">
        <header>
          <el-tag :type="row.type === 'price_change' ? 'warning' : row.type === 'promo' ? 'success' : ''">{{ announcementTypeLabel(row.type) }}</el-tag>
          <strong>{{ row.title }}</strong>
          <time>{{ row.publish_at || row.published_at || '' }}</time>
        </header>
        <p>{{ row.body }}</p>
        <small v-if="row.effective_from || row.effective_to">生效期：{{ row.effective_from || '不限' }} ~ {{ row.effective_to || '不限' }}</small>
      </article>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchMerchantAnnouncements, fetchMerchantMaintenanceStatus } from '../../api/modules'
import { announcementTypeLabel, maintenanceBlockText } from '../../utils/maintenance'

const rows = ref([])
const activePlan = ref(null)
const loading = ref(false)

const loadAll = async () => {
  loading.value = true
  try {
    const announcements = await fetchMerchantAnnouncements()
    rows.value = announcements?.data?.rows || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '加载公告失败')
  } finally {
    loading.value = false
  }
  try {
    const status = await fetchMerchantMaintenanceStatus()
    activePlan.value = status?.data?.plan || null
  } catch (err) {
    activePlan.value = null
  }
}

onMounted(loadAll)
</script>

<style scoped>
.merchant-announcements{display:grid;gap:18px}
.maintenance-banner{display:flex;gap:12px;align-items:center;padding:14px 20px;border:1px solid #fca5a5;border-radius:14px;background:#fef2f2;color:#7f1d1d;font-size:13px}
.page-card{padding:20px;border:1px solid #dce8f5;border-radius:18px;background:#fff}
.hero{display:flex;justify-content:space-between;align-items:center;gap:16px}
.eyebrow{color:#2563eb;font-size:12px;font-weight:900;letter-spacing:.14em}
.hero h1{margin:6px 0;font-size:24px}
.hero p{margin:0;color:#64748b;font-size:13px;line-height:1.6}
.announcement{padding:16px 0;border-top:1px solid #e2e8f0}
.announcement:first-child{border-top:0;padding-top:0}
.announcement header{display:flex;gap:10px;align-items:center}
.announcement header strong{flex:1;font-size:15px}
.announcement header time{color:#94a3b8;font-size:12px}
.announcement p{margin:10px 0 6px;color:#334155;font-size:14px;line-height:1.7;white-space:pre-wrap}
.announcement small{color:#94a3b8;font-size:12px}
.empty-hint{color:#94a3b8;font-size:13px;text-align:center;padding:12px 0}
</style>
