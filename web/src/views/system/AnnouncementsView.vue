<template>
  <div class="announcements-page">
    <section class="page-card hero">
      <div>
        <div class="eyebrow">ANNOUNCEMENTS</div>
        <h1>平台公告</h1>
        <p>真正的可发布公告：系统更新、活动促销、价格变动。草稿可改，已发布内容不可静默改写，错了请归档后重新发布。</p>
      </div>
      <el-button type="primary" @click="openCreate">新建公告</el-button>
    </section>

    <section class="page-card filters">
      <el-select v-model="filters.status" clearable placeholder="全部状态" @change="loadAnnouncements">
        <el-option v-for="item in ANNOUNCEMENT_STATUSES" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
      <el-button type="primary" @click="loadAnnouncements">刷新列表</el-button>
    </section>

    <section class="page-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ announcementTypeLabel(row.type) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'published' ? 'success' : row.status === 'archived' ? 'info' : 'warning'">{{ announcementStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="180">
          <template #default="{ row }">{{ row.publish_at || row.published_at || '—' }}</template>
        </el-table-column>
        <el-table-column label="生效期" width="320">
          <template #default="{ row }">{{ effectiveRange(row) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'draft'" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'draft'" size="small" type="success" @click="publish(row)">发布</el-button>
            <el-button v-if="row.status === 'published'" size="small" type="info" @click="archive(row)">归档</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p v-if="!loading && !rows.length" class="empty-hint">暂无公告，先新建一条草稿。</p>
    </section>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑草稿公告' : '新建公告'" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="类型">
          <el-select v-model="form.type">
            <el-option v-for="item in ANNOUNCEMENT_TYPES" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="正文">
          <el-input v-model="form.body" type="textarea" :rows="5" maxlength="5000" show-word-limit />
        </el-form-item>
        <el-form-item label="发布时间">
          <el-input v-model="form.publish_at" placeholder="YYYY-MM-DD HH:mm:ss，留空为立即" />
        </el-form-item>
        <el-form-item label="生效开始">
          <el-input v-model="form.effective_from" placeholder="YYYY-MM-DD HH:mm:ss，留空不限" />
        </el-form-item>
        <el-form-item label="生效结束">
          <el-input v-model="form.effective_to" placeholder="YYYY-MM-DD HH:mm:ss，留空不限" />
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
  archiveAdminAnnouncement,
  createAdminAnnouncement,
  fetchAdminAnnouncements,
  publishAdminAnnouncement,
  updateAdminAnnouncement,
} from '../../api/modules'
import {
  ANNOUNCEMENT_STATUSES,
  ANNOUNCEMENT_TYPES,
  announcementStatusLabel,
  announcementTypeLabel,
} from '../../utils/maintenance'

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref(null)
const filters = ref({ status: '' })
const form = ref({ type: 'system_update', title: '', body: '', publish_at: '', effective_from: '', effective_to: '' })

const effectiveRange = (row) => {
  const from = row.effective_from || '不限'
  const to = row.effective_to || '不限'
  if (from === '不限' && to === '不限') return '长期有效'
  return `${from} ~ ${to}`
}

const loadAnnouncements = async () => {
  loading.value = true
  try {
    const response = await fetchAdminAnnouncements({ status: filters.value.status || undefined, page: 1, page_size: 20 })
    rows.value = response?.data?.rows || []
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '加载公告列表失败')
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editingId.value = null
  form.value = { type: 'system_update', title: '', body: '', publish_at: '', effective_from: '', effective_to: '' }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingId.value = row.id
  form.value = {
    type: row.type,
    title: row.title,
    body: row.body,
    publish_at: row.publish_at || '',
    effective_from: row.effective_from || '',
    effective_to: row.effective_to || '',
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.title.trim() || !form.value.body.trim()) {
    ElMessage.warning('请填写标题与正文')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateAdminAnnouncement(editingId.value, form.value)
      ElMessage.success('草稿已更新')
    } else {
      await createAdminAnnouncement(form.value)
      ElMessage.success('草稿已创建，发布后商家可见')
    }
    dialogVisible.value = false
    await loadAnnouncements()
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '保存公告失败')
  } finally {
    saving.value = false
  }
}

const publish = async (row) => {
  try {
    await ElMessageBox.confirm(`发布「${row.title}」后商家即可看到，确定发布？`, '发布公告')
    await publishAdminAnnouncement(row.id)
    ElMessage.success('已发布')
    await loadAnnouncements()
  } catch (err) {
    if (err !== 'cancel') ElMessage.error(err?.response?.data?.message || '发布失败')
  }
}

const archive = async (row) => {
  try {
    await ElMessageBox.confirm(`归档「${row.title}」后商家不再看到，确定归档？`, '归档公告')
    await archiveAdminAnnouncement(row.id)
    ElMessage.success('已归档')
    await loadAnnouncements()
  } catch (err) {
    if (err !== 'cancel') ElMessage.error(err?.response?.data?.message || '归档失败')
  }
}

onMounted(loadAnnouncements)
</script>

<style scoped>
.announcements-page{display:grid;gap:18px}
.page-card{padding:20px;border:1px solid #dce8f5;border-radius:18px;background:#fff}
.hero{display:flex;justify-content:space-between;align-items:center;gap:16px}
.eyebrow{color:#2563eb;font-size:12px;font-weight:900;letter-spacing:.14em}
.hero h1{margin:6px 0;font-size:24px}
.hero p{margin:0;color:#64748b;font-size:13px;line-height:1.6}
.filters{display:flex;gap:12px;align-items:center}
.filters .el-select{width:180px}
.empty-hint{margin:14px 0 0;color:#94a3b8;font-size:13px;text-align:center}
</style>
