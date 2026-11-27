<template>
  <div class="maintenance-page">
    <section class="page-card hero">
      <div>
        <div class="eyebrow">MAINTENANCE</div>
        <h1>维护计划</h1>
        <p>维护窗口与在途订单安全联动：进行中的维护只拦截新下单，已下单的订单、支付与退款继续完成；窗口结束且未显式完成时不再拦截。</p>
      </div>
      <el-button type="primary" @click="openCreate">新建维护计划</el-button>
    </section>

    <section v-if="activePlan" class="page-card active-banner">
      <strong>维护进行中：{{ activePlan.title }}</strong>
      <span>窗口 {{ activePlan.window_start }} ~ {{ activePlan.window_end }} · 影响：{{ maintenanceModulesLabel(activePlan.affected_modules) }}</span>
      <el-button size="small" type="success" @click="setStatus(activePlan, 'completed')">完成维护</el-button>
    </section>

    <section class="page-card filters">
      <el-select v-model="filters.status" clearable placeholder="全部状态" @change="loadPlans">
        <el-option v-for="item in MAINTENANCE_STATUSES" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button type="primary" @click="loadAll">刷新列表</el-button>
    </section>

    <section class="page-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column label="窗口" width="340">
          <template #default="{ row }">{{ row.window_start }}<br />~ {{ row.window_end }}</template>
        </el-table-column>
        <el-table-column label="影响模块" width="140">
          <template #default="{ row }">{{ maintenanceModulesLabel(row.affected_modules) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'danger' : row.status === 'completed' ? 'success' : row.status === 'cancelled' ? 'info' : 'warning'">{{ maintenanceStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'draft'" size="small" @click="setStatus(row, 'scheduled')">排期</el-button>
            <el-button v-if="row.status === 'scheduled'" size="small" type="danger" @click="setStatus(row, 'active')">开始维护</el-button>
            <el-button v-if="row.status === 'active'" size="small" type="success" @click="setStatus(row, 'completed')">完成维护</el-button>
            <el-button v-if="['draft', 'scheduled', 'active'].includes(row.status)" size="small" @click="setStatus(row, 'cancelled')">取消</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p v-if="!loading && !rows.length" class="empty-hint">暂无维护计划。</p>
    </section>

    <el-dialog v-model="dialogVisible" title="新建维护计划" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="开始时间">
          <el-input v-model="form.window_start" placeholder="YYYY-MM-DD HH:mm:ss（北京时间）" />
        </el-form-item>
        <el-form-item label="结束时间">
          <el-input v-model="form.window_end" placeholder="YYYY-MM-DD HH:mm:ss（北京时间）" />
        </el-form-item>
        <el-form-item label="影响模块">
          <el-input v-model="form.affected_modules" placeholder="all 或 ordering,payment，留空为全部" />
        </el-form-item>
        <el-form-item label="关联公告 ID">
          <el-input v-model="form.announcement_id" placeholder="可选，提前告知商家的公告 ID" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createAdminMaintenancePlan,
  fetchAdminMaintenancePlans,
  fetchAdminMaintenanceStatus,
  setAdminMaintenancePlanStatus,
} from '../../api/modules'
import {
  MAINTENANCE_STATUSES,
  maintenanceModulesLabel,
  maintenanceStatusLabel,
} from '../../utils/maintenance'

const rows = ref([])
const activePlan = ref(null)
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const filters = ref({ status: '' })
const form = ref({ title: '', window_start: '', window_end: '', affected_modules: '', announcement_id: '' })

const loadPlans = async () => {
  loading.value = true
  try {
    const response = await fetchAdminMaintenancePlans({ status: filters.value.status || undefined, page: 1, page_size: 20 })
    rows.value = response?.data?.rows || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '加载维护计划失败')
  } finally {
    loading.value = false
  }
}

const loadStatus = async () => {
  try {
    const response = await fetchAdminMaintenanceStatus()
    activePlan.value = response?.data?.plan || null
  } catch (err) {
    activePlan.value = null
  }
}

const loadAll = async () => {
  await Promise.all([loadPlans(), loadStatus()])
}

const openCreate = () => {
  form.value = { title: '', window_start: '', window_end: '', affected_modules: '', announcement_id: '' }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.title.trim() || !form.value.window_start || !form.value.window_end) {
    ElMessage.warning('请填写标题与维护窗口')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: form.value.title,
      window_start: form.value.window_start,
      window_end: form.value.window_end,
      affected_modules: form.value.affected_modules || undefined,
    }
    if (form.value.announcement_id) payload.announcement_id = Number(form.value.announcement_id)
    await createAdminMaintenancePlan(payload)
    ElMessage.success('维护计划已创建（草稿），排期后按状态推进')
    dialogVisible.value = false
    await loadAll()
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '保存维护计划失败')
  } finally {
    saving.value = false
  }
}

const setStatus = async (row, status) => {
  const labels = { scheduled: '排期', active: '开始维护', completed: '完成维护', cancelled: '取消' }
  try {
    if (status === 'active') {
      await ElMessageBox.confirm(`开始维护后，新下单将被拦截（在途订单不受影响），确定开始「${row.title}」？`, '开始维护')
    } else {
      await ElMessageBox.confirm(`确定把「${row.title}」改为${labels[status]}？`, '变更维护状态')
    }
    await setAdminMaintenancePlanStatus(row.id, { status })
    ElMessage.success('状态已更新')
    await loadAll()
  } catch (err) {
    if (err !== 'cancel') ElMessage.error(err?.response?.data?.message || '变更状态失败')
  }
}

onMounted(loadAll)
</script>

<style scoped>
.maintenance-page{display:grid;gap:18px}
.page-card{padding:20px;border:1px solid #dce8f5;border-radius:18px;background:#fff}
.hero{display:flex;justify-content:space-between;align-items:center;gap:16px}
.eyebrow{color:#2563eb;font-size:12px;font-weight:900;letter-spacing:.14em}
.hero h1{margin:6px 0;font-size:24px}
.hero p{margin:0;color:#64748b;font-size:13px;line-height:1.6}
.active-banner{display:flex;align-items:center;gap:14px;border-color:#fca5a5;background:#fef2f2}
.active-banner span{flex:1;color:#7f1d1d;font-size:13px}
.filters{display:flex;gap:12px;align-items:center}
.filters .el-select{width:180px}
.empty-hint{margin:14px 0 0;color:#94a3b8;font-size:13px;text-align:center}
</style>
