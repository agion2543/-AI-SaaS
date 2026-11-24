<template>
  <div class="page-card">
    <div class="stores-toolbar">
      <div>
        <h2 class="page-title">门店管理</h2>
        <p class="stores-subtitle">维护门店信息、营业状态、营业时间和扫码点单入口。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增门店</el-button>
    </div>

    <section class="store-health-grid">
      <article v-for="item in storeHealthCards" :key="item.label" :class="['store-health-card', item.tone]">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.hint }}</small>
      </article>
    </section>

    <section class="store-mode-guide">
      <article>
        <span>顾客端展示</span>
        <strong>暂停接单仍可浏览菜单</strong>
        <small>暂停接单时顾客能看到商品、营业时间和暂停原因，但不能提交订单。</small>
      </article>
      <article>
        <span>下单流程</span>
        <strong>先提交后结算 / 下单即付款</strong>
        <small>前者适合先确认需求，后者适合快速成交；会影响顾客端主按钮和订单初始状态。</small>
      </article>
      <article>
        <span>接单方式</span>
        <strong>自动接单 / 手动接单</strong>
        <small>自动接单会让订单直接进入处理中，手动接单则进入待接单。</small>
      </article>
    </section>

    <el-table :data="stores">
      <el-table-column prop="name" label="门店名称" min-width="160" />
      <el-table-column prop="address" label="地址" min-width="200" />
      <el-table-column prop="contact_phone" label="联系电话" min-width="130" />
      <el-table-column label="营业状态" width="130">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' && row.is_open ? 'success' : 'info'">
            {{ row.status !== 'active' ? '已禁用' : row.is_open ? '接单中' : '暂停接单' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="business_hours" label="营业时间" min-width="150">
        <template #default="{ row }">{{ row.business_hours || '未设置' }}</template>
      </el-table-column>
      <el-table-column label="下单流程" min-width="170">
        <template #default="{ row }">
          <div class="mode-cell">
            <el-tag effect="plain">{{ orderModeLabel(row.order_mode) }}</el-tag>
            <small>{{ row.auto_accept ? '自动接单' : '手动接单' }}</small>
            <small class="mode-impact">{{ storeCustomerImpact(row) }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="顾客入口" min-width="230">
        <template #default="{ row }">
          <el-link type="primary" :href="row.qr_url" target="_blank">{{ row.qr_url }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="360" fixed="right">
        <template #default="{ row }">
          <div class="action-row">
            <el-button size="small" type="primary" plain @click="manageProducts(row)">管理商品</el-button>
            <el-button size="small" type="success" plain @click="openTables(row)">桌位与二维码</el-button>
            <el-button size="small" @click="openEdit(row)">编辑/营业设置</el-button>
            <el-button v-if="row.is_open" size="small" type="warning" @click="toggleOpen(row, false)">暂停接单</el-button>
            <el-button v-else size="small" type="success" @click="toggleOpen(row, true)">恢复接单</el-button>
            <el-button v-if="row.status === 'active'" size="small" type="danger" @click="disableStore(row)">禁用</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="stores-pagination">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="changePage"
      />
    </div>

    <el-dialog v-model="tablesDialogVisible" :title="`${tablesStore?.name || '门店'} · 扫码点餐桌位`" width="900px" class="tables-dialog">
      <div class="tables-toolbar">
        <div>
          <strong>桌位数量</strong>
          <p>设置后生成 1 至 N 桌的专属二维码，可为每桌自定义标签（如 A01、包间）。标签只用于展示，不改变二维码身份；请将二维码张贴在对应桌位。</p>
        </div>
        <div class="tables-count-control">
          <el-input-number v-model="tableCount" :min="0" :max="300" :step="1" :precision="0" controls-position="right" :disabled="tablesSaving" />
          <el-button type="primary" :loading="tablesSaving" @click="saveTables">保存桌位设置</el-button>
        </div>
      </div>
      <div v-loading="tablesLoading" class="table-qr-grid">
        <article v-for="item in tableQrs" :key="item.tableNo" class="table-qr-card">
          <strong>{{ item.label || `${item.tableNo}号桌` }}</strong>
          <el-input v-model="item.label" :placeholder="`${item.tableNo}号桌`" maxlength="64" size="small" aria-label="桌位标签" />
          <img :src="item.image" :alt="`${item.label || `${item.tableNo}号桌`}扫码点餐二维码`" />
          <div class="table-qr-actions">
            <el-button size="small" @click="downloadTableQr(item)">下载二维码</el-button>
            <el-button size="small" @click="printTableQr(item)">打印</el-button>
          </div>
        </article>
        <el-empty v-if="!tablesLoading && tableQrs.length === 0" description="当前未设置桌位" />
      </div>
      <template #footer>
        <el-button :disabled="!tableQrs.length" @click="printAllTables">打印全部桌码</el-button>
        <el-button @click="tablesDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新增门店' : '编辑门店与营业设置'" width="560px">
      <el-form :model="form" label-position="top">
        <el-form-item label="门店名称">
          <el-input v-model="form.name" maxlength="120" placeholder="请输入门店名称" />
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="form.address" placeholder="请输入门店地址" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="form.contact_phone" maxlength="11" placeholder="请输入 11 位联系电话" />
        </el-form-item>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="基础状态">
              <el-select v-model="form.status">
                <el-option label="启用" value="active" />
                <el-option label="禁用" value="inactive" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="是否接单">
              <el-switch v-model="form.is_open" active-text="接单中" inactive-text="暂停" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="营业时间">
          <el-input v-model="form.business_hours" placeholder="例如：周一至周日 09:00-22:00" maxlength="120" />
        </el-form-item>
        <el-form-item label="下单流程">
          <el-radio-group v-model="form.order_mode">
            <el-radio-button label="submit_later">先提交后结算</el-radio-button>
            <el-radio-button label="pay_first">下单即付款</el-radio-button>
          </el-radio-group>
          <small class="field-tip">先提交后结算适合需要分阶段确认的服务；下单即付款适合快速成交和带走类服务。</small>
        </el-form-item>
        <el-form-item label="接单方式">
          <el-switch v-model="form.auto_accept" active-text="自动接单" inactive-text="手动接单" />
          <small class="field-tip">自动接单会在顾客提交后直接进入处理中；手动接单需要商家确认。</small>
        </el-form-item>
        <el-form-item label="暂停原因">
          <el-input v-model="form.pause_reason" type="textarea" :rows="2" placeholder="暂停接单时给顾客看的说明" maxlength="255" />
        </el-form-item>
        <section class="setting-preview" :class="settingPreview.tone">
          <span>配置生效后</span>
          <strong>{{ settingPreview.title }}</strong>
          <p>{{ settingPreview.text }}</p>
          <small>{{ settingPreview.orderText }}</small>
        </section>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">{{ dialogMode === 'create' ? '创建门店' : '保存修改' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import QRCode from 'qrcode'
import { ElMessage } from 'element-plus'
import { configureMerchantStoreTables, createMerchantStore, deleteMerchantStore, fetchMerchantStoreTables, fetchMerchantStores, updateMerchantStore } from '../../api/modules'

const router = useRouter()
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const dialogVisible = ref(false)
const dialogMode = ref('create')
const tablesDialogVisible = ref(false)
const tablesLoading = ref(false)
const tablesSaving = ref(false)
const tablesStore = ref(null)
const tableCount = ref(0)
const tableQrs = ref([])
const editingId = ref(null)
const form = reactive({
  name: '',
  address: '',
  contact_phone: '',
  status: 'active',
  is_open: true,
  business_hours: '',
  pause_reason: '',
  order_mode: 'submit_later',
  auto_accept: true
})

const enabledStores = computed(() => stores.value.filter((item) => item.status === 'active'))
const openStores = computed(() => enabledStores.value.filter((item) => item.is_open !== false))
const pausedStores = computed(() => enabledStores.value.filter((item) => item.is_open === false))
const manualAcceptStores = computed(() => enabledStores.value.filter((item) => !item.auto_accept))
const storeHealthCards = computed(() => [
  { label: '门店总数', value: stores.value.length, hint: '当前商家已维护门店。', tone: 'primary' },
  { label: '接单中', value: openStores.value.length, hint: '顾客端可提交订单。', tone: 'success' },
  { label: '暂停接单', value: pausedStores.value.length, hint: '顾客可浏览，但不能下单。', tone: pausedStores.value.length ? 'warning' : 'muted' },
  { label: '手动接单', value: manualAcceptStores.value.length, hint: '订单需要商家确认后处理。', tone: manualAcceptStores.value.length ? 'warning' : 'muted' }
])
const settingPreview = computed(() => {
  if (form.status !== 'active') {
    return {
      tone: 'muted',
      title: '顾客端暂不可访问该门店',
      text: '门店禁用后，顾客扫码会看到不可访问提示，适合长期停用或测试门店。',
      orderText: '不会创建新的顾客订单。'
    }
  }
  if (!form.is_open) {
    return {
      tone: 'warning',
      title: '顾客可浏览菜单，但不能提交订单',
      text: form.pause_reason.trim() || '顾客端会展示暂停接单提示，建议填写清晰原因。',
      orderText: '不会创建新的顾客订单。'
    }
  }
  if (form.order_mode === 'submit_later') {
    return {
      tone: 'success',
      title: '顾客点击“提交订单”',
      text: '订单先进入系统，顾客可在订单状态页查看进度，后续再结算。',
      orderText: form.auto_accept ? '订单创建后直接进入处理中。' : '订单创建后进入待接单。'
    }
  }
  return {
    tone: 'primary',
    title: '顾客点击“提交并付款”',
    text: '订单创建后进入付款流程，适合快速成交和标准商品。',
    orderText: form.auto_accept ? '付款确认后订单进入处理中。' : '付款确认后订单进入待接单。'
  }
})

const resetForm = () => {
  Object.assign(form, {
    name: '',
    address: '',
    contact_phone: '',
    status: 'active',
    is_open: true,
    business_hours: '',
    pause_reason: '',
    order_mode: 'submit_later',
    auto_accept: true
  })
  editingId.value = null
}

const load = async () => {
  const res = await fetchMerchantStores({ page: page.value, page_size: pageSize })
  stores.value = res.data.list || []
  total.value = res.data.total || 0
}

const validateForm = () => {
  if (!form.name.trim()) {
    ElMessage.error('请输入门店名称')
    return false
  }
  if (!/^1\d{10}$/.test(form.contact_phone.trim())) {
    ElMessage.error('联系电话必须为 11 位数字')
    return false
  }
  return true
}

const openCreate = () => {
  dialogMode.value = 'create'
  resetForm()
  dialogVisible.value = true
}

const openEdit = (row) => {
  dialogMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    name: row.name || '',
    address: row.address || '',
    contact_phone: row.contact_phone || '',
    status: row.status || 'active',
    is_open: row.is_open !== false,
    business_hours: row.business_hours || '',
    pause_reason: row.pause_reason || '',
    order_mode: row.order_mode || 'pay_first',
    auto_accept: Boolean(row.auto_accept)
  })
  dialogVisible.value = true
}

const payload = () => ({
  name: form.name.trim(),
  address: form.address.trim(),
  contact_phone: form.contact_phone.trim(),
  status: form.status,
  is_open: form.is_open,
  business_hours: form.business_hours.trim(),
  pause_reason: form.pause_reason.trim(),
  order_mode: form.order_mode,
  auto_accept: form.auto_accept
})

const submit = async () => {
  if (!validateForm()) return
  try {
    if (dialogMode.value === 'create') {
      await createMerchantStore(payload())
      ElMessage.success('门店已创建')
    } else {
      await updateMerchantStore(editingId.value, payload())
      ElMessage.success('门店信息已更新')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '操作失败')
  }
}

const manageProducts = (row) => {
  router.push(`/merchant/stores/${row.id}/products`)
}
const renderTableQrs = async (tables = []) => {
  tableQrs.value = await Promise.all(tables.map(async (table) => ({
    tableNo: table.table_no,
    label: String(table.table_label || `${table.table_no}号桌`),
    url: table.url,
    image: await QRCode.toDataURL(table.url, { errorCorrectionLevel: 'M', margin: 2, width: 220 })
  })))
}
const openTables = async (row) => {
  tablesStore.value = row
  tablesDialogVisible.value = true
  tablesLoading.value = true
  tableQrs.value = []
  try {
    const res = await fetchMerchantStoreTables(row.id)
    const config = res.data || {}
    tableCount.value = Number(config.table_count) || 0
    await renderTableQrs(config.tables || [])
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '读取桌位设置失败')
  } finally {
    tablesLoading.value = false
  }
}
const saveTables = async () => {
  if (!tablesStore.value || !Number.isInteger(tableCount.value) || tableCount.value < 0 || tableCount.value > 300) {
    ElMessage.error('桌数需为 0 到 300 的整数')
    return
  }
  tablesSaving.value = true
  try {
    await configureMerchantStoreTables(tablesStore.value.id, {
      table_count: tableCount.value,
      tables: tableQrs.value.filter((item) => Number(item.tableNo) <= tableCount.value).map((item) => ({ table_no: item.tableNo, table_label: String(item.label || `${item.tableNo}号桌`).trim() || `${item.tableNo}号桌` }))
    })
    const latest = await fetchMerchantStoreTables(tablesStore.value.id)
    const config = latest.data || {}
    tableCount.value = Number(config.table_count) || 0
    await renderTableQrs(config.tables || [])
    ElMessage.success('桌位设置已保存并刷新')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '保存桌位设置失败')
  } finally {
    tablesSaving.value = false
  }
}
const downloadTableQr = (item) => {
  const link = document.createElement('a')
  link.href = item.image
  link.download = `桌位-${(item.label || `${item.tableNo}号桌`).replace(/[\\/:*?"<>|]/g, "_")}-扫码点餐.png`
  link.click()
}
const printTableQr = (item) => printTableQrItems([item])
const printAllTables = () => printTableQrItems(tableQrs.value)
const escapeHtml = (value) => String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll("\"", "&quot;").replaceAll("'", "&#39;")
const printTableQrItems = (items) => {
  if (!items.length) return
  const printWindow = window.open('', '_blank')
  if (!printWindow) {
    ElMessage.warning('浏览器阻止了打印窗口，请允许弹出窗口后重试')
    return
  }
  const cards = items.map((item) => `<article><h2>${escapeHtml(item.label || `${item.tableNo}号桌`)}</h2><img src="${item.image}" alt="桌位二维码"><p>扫码点餐</p></article>`).join('')
  printWindow.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>扫码点餐桌码</title><style>body{font-family:Arial,sans-serif}.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}article{text-align:center;border:1px solid #ddd;padding:16px;break-inside:avoid}img{width:180px;height:180px}h2{margin:0 0 10px}p{margin:8px 0}@media print{.grid{gap:10px}}</style></head><body><main class="grid">${cards}</main><script>window.onload=()=>window.print()<\/script></body></html>`)
  printWindow.document.close()
}

const toggleOpen = async (row, isOpen) => {
  await updateMerchantStore(row.id, {
    name: row.name,
    address: row.address,
    contact_phone: row.contact_phone,
    status: row.status,
    is_open: isOpen,
    business_hours: row.business_hours || '',
    pause_reason: isOpen ? '' : (row.pause_reason || '商家暂停接单'),
    order_mode: row.order_mode || 'pay_first',
    auto_accept: Boolean(row.auto_accept)
  })
  ElMessage.success(isOpen ? '已恢复接单' : '已暂停接单')
  load()
}

const disableStore = async (row) => {
  await deleteMerchantStore(row.id)
  ElMessage.success('门店已禁用')
  load()
}

const changePage = async (nextPage) => {
  page.value = nextPage
  await load()
}

onMounted(load)

const orderModeLabel = (value) => ({
  submit_later: '先提交后结算',
  pay_first: '下单即付款'
}[value] || '下单即付款')

const storeCustomerImpact = (row) => {
  if (row.status !== 'active') return '顾客端不可访问'
  if (row.is_open === false) return '可浏览，不可下单'
  if (row.order_mode === 'submit_later') return row.auto_accept ? '提交后进入处理中' : '提交后等待接单'
  return row.auto_accept ? '付款后进入处理中' : '付款后等待接单'
}
</script>

<style scoped>
.stores-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 18px;
}

.stores-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
}

.stores-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.store-health-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.store-health-card {
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.store-health-card.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.store-health-card.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.store-health-card.muted {
  border-color: #e2e8f0;
  background: #f8fafc;
}

.store-health-card span,
.store-health-card strong,
.store-health-card small {
  display: block;
}

.store-health-card span {
  color: #64748b;
  font-size: 12px;
}

.store-health-card strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 24px;
}

.store-health-card small {
  margin-top: 4px;
  color: #64748b;
  line-height: 1.45;
}

.store-mode-guide {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.store-mode-guide article {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.store-mode-guide span,
.store-mode-guide strong,
.store-mode-guide small {
  display: block;
}

.store-mode-guide span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
}

.store-mode-guide strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 16px;
}

.store-mode-guide small {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.55;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.mode-cell {
  display: grid;
  gap: 5px;
  justify-items: start;
}

.mode-cell small,
.field-tip {
  color: var(--muted);
  line-height: 1.5;
}

.mode-cell .mode-impact {
  color: #2563eb;
  font-weight: 700;
}

.field-tip {
  display: block;
  margin-top: 6px;
}

.setting-preview {
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.setting-preview.success {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.setting-preview.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.setting-preview.muted {
  border-color: #e2e8f0;
  background: #f8fafc;
}

.setting-preview span,
.setting-preview strong,
.setting-preview p,
.setting-preview small {
  display: block;
}

.setting-preview span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.setting-preview strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 17px;
}

.setting-preview p {
  margin: 6px 0 0;
  color: #475569;
  line-height: 1.6;
}

.setting-preview small {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.5;
}

@media (max-width: 720px) {
  .stores-toolbar {
    flex-direction: column;
  }

  .store-health-grid,
  .store-mode-guide {
    grid-template-columns: 1fr;
  }
}
.tables-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  padding: 14px;
  margin-bottom: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}
.tables-toolbar p {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.5;
}
.tables-count-control,
.table-qr-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.table-qr-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 14px;
  max-height: 60vh;
  overflow: auto;
  min-height: 120px;
}
.table-qr-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.table-qr-card img {
  width: 180px;
  height: 180px;
  object-fit: contain;
}
.table-qr-actions {
  justify-content: center;
  flex-wrap: wrap;
}
@media (max-width: 720px) {
  .tables-toolbar {
    align-items: stretch;
    flex-direction: column;
  }
  .tables-count-control {
    flex-wrap: wrap;
  }
}
</style>
