<template>
  <div class="page-card">
    <div class="stores-toolbar">
      <div>
        <h2 class="page-title">门店管理</h2>
        <p class="stores-subtitle">维护门店信息、营业状态、营业时间和扫码点单入口。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增门店</el-button>
    </div>

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
      <el-table-column label="顾客入口" min-width="230">
        <template #default="{ row }">
          <el-link type="primary" :href="row.qr_url" target="_blank">{{ row.qr_url }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="360" fixed="right">
        <template #default="{ row }">
          <div class="action-row">
            <el-button size="small" type="primary" plain @click="manageProducts(row)">管理商品</el-button>
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
        <el-form-item label="暂停原因">
          <el-input v-model="form.pause_reason" type="textarea" :rows="2" placeholder="暂停接单时给顾客看的说明" maxlength="255" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">{{ dialogMode === 'create' ? '创建门店' : '保存修改' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createMerchantStore, deleteMerchantStore, fetchMerchantStores, updateMerchantStore } from '../../api/modules'

const router = useRouter()
const stores = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref(null)
const form = reactive({
  name: '',
  address: '',
  contact_phone: '',
  status: 'active',
  is_open: true,
  business_hours: '',
  pause_reason: ''
})

const resetForm = () => {
  Object.assign(form, {
    name: '',
    address: '',
    contact_phone: '',
    status: 'active',
    is_open: true,
    business_hours: '',
    pause_reason: ''
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
    pause_reason: row.pause_reason || ''
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
  pause_reason: form.pause_reason.trim()
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

const toggleOpen = async (row, isOpen) => {
  await updateMerchantStore(row.id, {
    name: row.name,
    address: row.address,
    contact_phone: row.contact_phone,
    status: row.status,
    is_open: isOpen,
    business_hours: row.business_hours || '',
    pause_reason: isOpen ? '' : (row.pause_reason || '商家暂停接单')
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

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .stores-toolbar {
    flex-direction: column;
  }
}
</style>
