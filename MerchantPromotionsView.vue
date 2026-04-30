<template>
  <div class="promotion-page">
    <section class="promotion-hero page-card">
      <div>
        <div class="eyebrow">MARKETING</div>
        <h2>优惠活动</h2>
        <p>把 AI 经营建议落到真实活动里，顾客扫码门店页会自动展示正在发布的优惠。</p>
      </div>
      <el-button type="primary" size="large" @click="openCreate">新增活动</el-button>
    </section>

    <section class="page-card promotion-table-card">
      <el-table v-loading="loading" :data="promotions" empty-text="暂无优惠活动">
        <el-table-column prop="title" label="活动名称" min-width="180" />
        <el-table-column label="活动类型" width="110">
          <template #default="{ row }">
            <el-tag :type="row.type === 'discount' ? 'warning' : 'success'">
              {{ row.type === 'discount' ? '折扣' : '满减' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="适用门店" min-width="150">
          <template #default="{ row }">
            {{ row.store?.name || '全部门店' }}
          </template>
        </el-table-column>
        <el-table-column label="优惠力度" min-width="170">
          <template #default="{ row }">
            {{ formatDiscount(row) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="210">
          <template #default="{ row }">
            {{ formatDate(row.valid_from) }} - {{ formatDate(row.valid_to) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="openEdit(row)">编辑</el-button>
              <el-button
                v-if="row.status !== 'published'"
                size="small"
                type="success"
                @click="changeStatus(row, 'published')"
              >
                发布
              </el-button>
              <el-button v-else size="small" type="warning" @click="changeStatus(row, 'inactive')">停用</el-button>
              <el-button size="small" type="danger" plain @click="removePromotion(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="handlePageChange"
        />
      </div>
    </section>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑优惠活动' : '新增优惠活动'" width="640px">
      <el-form label-position="top">
        <el-form-item label="活动名称">
          <el-input v-model="form.title" maxlength="120" placeholder="例如：新客到店立减活动" />
        </el-form-item>

        <el-form-item label="活动说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="500"
            placeholder="写清楚适用规则，方便顾客扫码后理解"
          />
        </el-form-item>

        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="适用门店">
              <el-select v-model="form.store_id" clearable placeholder="全部门店">
                <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-select v-model="form.status">
                <el-option label="草稿" value="draft" />
                <el-option label="发布中" value="published" />
                <el-option label="已停用" value="inactive" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="活动类型">
          <el-radio-group v-model="form.type" class="type-switch">
            <el-radio-button label="amount">满减活动</el-radio-button>
            <el-radio-button label="discount">折扣活动</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <div class="discount-box">
          <el-row :gutter="14">
            <el-col :span="12">
              <el-form-item label="门槛金额（元）">
                <el-input-number v-model="form.threshold_yuan" :min="0" :precision="2" :step="10" class="full-input" />
              </el-form-item>
            </el-col>
            <el-col v-if="form.type === 'amount'" :span="12">
              <el-form-item label="优惠金额（元）">
                <el-input-number v-model="form.discount_yuan" :min="0" :precision="2" :step="5" class="full-input" />
              </el-form-item>
            </el-col>
            <el-col v-else :span="12">
              <el-form-item label="自定义折率">
                <el-input-number
                  v-model="form.discount_rate"
                  :min="1"
                  :max="99"
                  :step="1"
                  class="full-input"
                />
                <div class="form-tip">例如填写 85 表示 8.5 折，填写 70 表示 7 折。</div>
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="开始日期">
              <el-date-picker v-model="form.valid_from" type="date" value-format="YYYY-MM-DD" placeholder="不限" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="结束日期">
              <el-date-picker v-model="form.valid_to" type="date" value-format="YYYY-MM-DD" placeholder="不限" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePromotion">保存活动</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createMerchantPromotion,
  deleteMerchantPromotion,
  fetchMerchantPromotions,
  fetchMerchantStores,
  updateMerchantPromotion
} from '../../api/modules'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const promotions = ref([])
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const editingId = ref(null)

const form = reactive({
  store_id: null,
  title: '',
  description: '',
  type: 'amount',
  threshold_yuan: 0,
  discount_yuan: 0,
  discount_rate: 85,
  status: 'draft',
  valid_from: '',
  valid_to: ''
})

const centsToYuan = (value) => Number((Number(value || 0) / 100).toFixed(2))
const yuanToCents = (value) => Math.round(Number(value || 0) * 100)

const formatMoney = (value) => {
  const yuan = centsToYuan(value)
  return yuan % 1 === 0 ? yuan.toFixed(0) : yuan.toFixed(2)
}

const formatRate = (value) => {
  const rate = Number(value || 0)
  if (!rate) return '折扣待设置'
  return `${rate / 10} 折`
}

const formatDiscount = (row) => {
  if (row.type === 'discount') {
    const threshold = row.threshold ? `满 ${formatMoney(row.threshold)} 元` : '无门槛'
    return `${threshold} 享 ${formatRate(row.discount_rate)}`
  }
  if (!row.discount) return '到店咨询'
  if (!row.threshold) return `立减 ${formatMoney(row.discount)} 元`
  return `满 ${formatMoney(row.threshold)} 减 ${formatMoney(row.discount)} 元`
}

const formatDate = (value) => {
  if (!value) return '不限'
  return String(value).slice(0, 10)
}

const statusLabel = (status) => ({
  draft: '草稿',
  published: '发布中',
  inactive: '已停用'
}[status] || status)

const statusType = (status) => ({
  draft: 'info',
  published: 'success',
  inactive: 'warning'
}[status] || 'info')

const resetForm = () => {
  editingId.value = null
  Object.assign(form, {
    store_id: null,
    title: '',
    description: '',
    type: 'amount',
    threshold_yuan: 0,
    discount_yuan: 0,
    discount_rate: 85,
    status: 'draft',
    valid_from: '',
    valid_to: ''
  })
}

const loadPromotions = async () => {
  loading.value = true
  try {
    const res = await fetchMerchantPromotions({ page: page.value, page_size: pageSize })
    promotions.value = res.data.list || res.data.items || []
    total.value = res.data.total || 0
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '优惠活动加载失败')
  } finally {
    loading.value = false
  }
}

const loadStores = async () => {
  try {
    const res = await fetchMerchantStores({ page: 1, page_size: 100 })
    stores.value = res.data.list || res.data.items || []
  } catch {
    stores.value = []
  }
}

const openCreate = () => {
  resetForm()
  dialogVisible.value = true
}

const openEdit = (row) => {
  editingId.value = row.id
  Object.assign(form, {
    store_id: row.store_id || null,
    title: row.title || '',
    description: row.description || '',
    type: row.type || 'amount',
    threshold_yuan: centsToYuan(row.threshold),
    discount_yuan: centsToYuan(row.discount),
    discount_rate: row.discount_rate || 85,
    status: row.status || 'draft',
    valid_from: formatDate(row.valid_from) === '不限' ? '' : formatDate(row.valid_from),
    valid_to: formatDate(row.valid_to) === '不限' ? '' : formatDate(row.valid_to)
  })
  dialogVisible.value = true
}

const buildPayload = (override = {}) => ({
  store_id: form.store_id || null,
  title: form.title,
  description: form.description,
  type: form.type,
  threshold: yuanToCents(form.threshold_yuan),
  discount: form.type === 'amount' ? yuanToCents(form.discount_yuan) : 0,
  discount_rate: form.type === 'discount' ? Number(form.discount_rate || 0) : 0,
  status: form.status,
  valid_from: form.valid_from || '',
  valid_to: form.valid_to || '',
  ...override
})

const savePromotion = async () => {
  saving.value = true
  try {
    if (editingId.value) {
      await updateMerchantPromotion(editingId.value, buildPayload())
    } else {
      await createMerchantPromotion(buildPayload())
    }
    ElMessage.success('优惠活动已保存')
    dialogVisible.value = false
    await loadPromotions()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const buildPayloadFromRow = (row, override = {}) => ({
  store_id: row.store_id || null,
  title: row.title,
  description: row.description,
  type: row.type || 'amount',
  threshold: row.threshold || 0,
  discount: row.discount || 0,
  discount_rate: row.discount_rate || 0,
  status: row.status || 'draft',
  valid_from: formatDate(row.valid_from) === '不限' ? '' : formatDate(row.valid_from),
  valid_to: formatDate(row.valid_to) === '不限' ? '' : formatDate(row.valid_to),
  ...override
})

const changeStatus = async (row, status) => {
  try {
    await updateMerchantPromotion(row.id, buildPayloadFromRow(row, { status }))
    ElMessage.success(status === 'published' ? '活动已发布' : '活动已停用')
    await loadPromotions()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '操作失败')
  }
}

const removePromotion = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除“${row.title}”吗？删除后不可在列表中恢复。`, '删除优惠活动', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await deleteMerchantPromotion(row.id)
    ElMessage.success('活动已删除')
    await loadPromotions()
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

const handlePageChange = (nextPage) => {
  page.value = nextPage
  loadPromotions()
}

onMounted(() => {
  loadPromotions()
  loadStores()
})
</script>

<style scoped>
.promotion-page {
  display: grid;
  gap: 20px;
}

.promotion-hero {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at 88% 18%, rgba(34, 197, 94, 0.2), transparent 24%),
    linear-gradient(135deg, #ffffff 0%, #f0fdf4 100%);
}

.eyebrow {
  color: #16a34a;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.promotion-hero h2 {
  margin: 8px 0;
  font-size: 30px;
}

.promotion-hero p {
  margin: 0;
  max-width: 680px;
  color: #64748b;
  line-height: 1.7;
}

.promotion-table-card {
  padding: 18px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.type-switch {
  width: 100%;
}

.discount-box {
  margin-bottom: 18px;
  padding: 14px 12px 4px;
  border: 2px solid #ff4d4f;
  border-radius: 8px;
  background: #fffafa;
}

.full-input {
  width: 100%;
}

.form-tip {
  margin-top: 8px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 760px) {
  .promotion-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
