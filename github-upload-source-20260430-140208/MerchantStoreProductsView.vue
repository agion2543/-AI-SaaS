<template>
  <div class="page-card">
    <div class="products-toolbar">
      <div>
        <el-button text class="back-button" @click="router.push('/merchant/stores')">返回门店管理</el-button>
        <h2 class="page-title">商品管理</h2>
        <p class="products-subtitle">维护当前门店商品/服务，分类会同步用于顾客端点单展示。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增商品</el-button>
    </div>

    <el-table :data="products">
      <el-table-column prop="name" label="商品名称" min-width="180" />
      <el-table-column prop="category" label="分类" width="130" />
      <el-table-column label="价格" width="120">
        <template #default="{ row }">¥{{ centsToYuan(row.price) }}</template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="90" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'">
            {{ row.status === 'active' ? '启用中' : '已禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
      <el-table-column label="操作" width="230">
        <template #default="{ row }">
          <div class="action-row">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'active'" size="small" type="danger" @click="disableProduct(row)">禁用</el-button>
            <el-button v-else size="small" type="success" @click="enableProduct(row)">启用</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="products-pagination">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="changePage"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新增商品' : '编辑商品'" width="560px">
      <el-form :model="form" label-position="top">
        <el-form-item label="商品名称">
          <el-input v-model="form.name" placeholder="例如：招牌套餐、基础护理、单次咨询" maxlength="120" />
        </el-form-item>
        <el-form-item label="商品分类">
          <el-input v-model="form.category" placeholder="例如：套餐、饮品、服务、热销" maxlength="80" />
        </el-form-item>
        <el-form-item label="价格（元）">
          <el-input-number v-model="form.price_yuan" :min="0" :precision="2" :step="1" class="wide-input" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="简要说明商品或服务内容" maxlength="500" />
        </el-form-item>
        <el-form-item label="图片 URL">
          <el-input v-model="form.image_url" placeholder="可选，后续可替换为上传组件" maxlength="500" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="1" :step="1" class="wide-input" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">{{ dialogMode === 'create' ? '创建商品' : '保存修改' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  createMerchantStoreProduct,
  deleteMerchantStoreProduct,
  fetchMerchantStoreProducts,
  updateMerchantStoreProduct
} from '../../api/modules'

const route = useRoute()
const router = useRouter()
const storeId = Number(route.params.storeId)
const products = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref(null)

const form = reactive({
  name: '',
  category: '默认分类',
  price_yuan: 0,
  description: '',
  image_url: '',
  sort: 100
})

const centsToYuan = (value) => ((Number(value || 0) / 100).toFixed(2))
const yuanToCents = (value) => Math.round(Number(value || 0) * 100)

const load = async () => {
  const res = await fetchMerchantStoreProducts(storeId, { page: page.value, page_size: pageSize })
  products.value = res.data.list || []
  total.value = res.data.total || 0
}

const resetForm = () => {
  Object.assign(form, {
    name: '',
    category: '默认分类',
    price_yuan: 0,
    description: '',
    image_url: '',
    sort: 100
  })
  editingId.value = null
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
    category: row.category || '默认分类',
    price_yuan: Number(row.price || 0) / 100,
    description: row.description || '',
    image_url: row.image_url || '',
    sort: row.sort || 100
  })
  dialogVisible.value = true
}

const buildPayload = (status = 'active') => ({
  store_id: storeId,
  name: form.name.trim(),
  category: form.category.trim() || '默认分类',
  price: yuanToCents(form.price_yuan),
  description: form.description.trim(),
  image_url: form.image_url.trim(),
  sort: Number(form.sort || 100),
  status
})

const validateForm = () => {
  if (!form.name.trim()) {
    ElMessage.warning('请输入商品名称')
    return false
  }
  if (Number(form.price_yuan) < 0) {
    ElMessage.warning('价格不能为负数')
    return false
  }
  return true
}

const submit = async () => {
  if (!validateForm()) return
  try {
    if (dialogMode.value === 'create') {
      await createMerchantStoreProduct(buildPayload('active'))
      ElMessage.success('商品已创建')
    } else {
      const current = products.value.find((item) => item.id === editingId.value)
      await updateMerchantStoreProduct(editingId.value, buildPayload(current?.status || 'active'))
      ElMessage.success('商品已更新')
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '操作失败')
  }
}

const disableProduct = async (row) => {
  await deleteMerchantStoreProduct(row.id)
  ElMessage.success('商品已禁用')
  await load()
}

const enableProduct = async (row) => {
  await updateMerchantStoreProduct(row.id, {
    store_id: storeId,
    name: row.name,
    category: row.category || '默认分类',
    price: row.price,
    description: row.description || '',
    image_url: row.image_url || '',
    sort: row.sort || 100,
    status: 'active'
  })
  ElMessage.success('商品已启用')
  await load()
}

const changePage = async (nextPage) => {
  page.value = nextPage
  await load()
}

onMounted(load)
</script>

<style scoped>
.products-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 18px;
}

.back-button {
  margin-bottom: 4px;
  padding-left: 0;
}

.products-subtitle {
  margin: 6px 0 0;
  color: var(--muted);
}

.products-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.wide-input {
  width: 100%;
}
</style>
