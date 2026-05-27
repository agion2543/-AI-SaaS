<template>
  <div class="products-page">
    <PageHero
      eyebrow="PRODUCT CENTER"
      title="商品管理"
      description="维护当前门店的商品、分类、图片、价格、上下架和排序。顾客扫码点单页会实时同步展示。"
      compact
    >
      <template #actions>
        <el-button plain @click="router.push('/merchant/stores')">返回门店管理</el-button>
        <el-button plain @click="openCustomerPreview">预览顾客端</el-button>
        <el-button plain @click="openProductAI">AI 优化菜单</el-button>
        <el-button type="primary" @click="openCreate">新增商品</el-button>
      </template>
    </PageHero>

    <section class="metric-grid">
      <MetricCard label="商品总数" :value="products.length" hint="当前门店已维护商品数量。" tone="primary" />
      <MetricCard label="上架商品" :value="activeCount" hint="顾客扫码页可见的商品。" tone="success" />
      <MetricCard label="商品分类" :value="categoryOptions.length" hint="用于顾客按分类点单。" tone="cyan" />
      <MetricCard label="平均价格" :value="formatMoney(avgPrice)" hint="当前商品平均售价。" tone="warning" />
      <MetricCard label="开卖完成度" :value="`${launchScore}%`" hint="图片、描述、分类、上架、排序综合评分。" />
    </section>

    <section class="ops-focus">
      <button
        v-for="item in primaryOpsCards"
        :key="item.key"
        type="button"
        :class="['ops-card', item.tone]"
        @click="item.action"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.hint }}</small>
      </button>
      <el-collapse class="ops-more">
        <el-collapse-item title="展开查看其他菜单问题" name="ops">
          <div class="ops-grid compact">
            <button
              v-for="item in secondaryOpsCards"
              :key="item.key"
              type="button"
              :class="['ops-card', item.tone]"
              @click="item.action"
            >
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
              <small>{{ item.hint }}</small>
            </button>
          </div>
        </el-collapse-item>
      </el-collapse>
    </section>

    <section class="launch-board" :class="launchTone">
      <div class="launch-summary">
        <span>MENU LAUNCH CHECK</span>
        <strong>{{ launchBrief.title }}</strong>
        <p>{{ launchBrief.text }}</p>
      </div>
      <div class="launch-checks">
        <div v-for="item in launchChecks" :key="item.label" :class="{ done: item.done }">
          <span>{{ item.done ? '已完成' : '待优化' }}</span>
          <strong>{{ item.label }}</strong>
          <small>{{ item.hint }}</small>
        </div>
      </div>
    </section>

    <section class="category-strip">
      <button
        type="button"
        :class="{ active: !filters.category }"
        @click="setCategoryFilter('')"
      >
        <span>全部分类</span>
        <strong>{{ products.length }}</strong>
      </button>
      <button
        v-for="item in categoryStats"
        :key="item.name"
        type="button"
        :class="{ active: filters.category === item.name }"
        @click="setCategoryFilter(item.name)"
      >
        <span>{{ item.name }}</span>
        <strong>{{ item.total }}</strong>
        <small>{{ item.active }} 个上架</small>
      </button>
    </section>

    <DataPanel
      eyebrow="FILTER"
      title="商品列表"
      description="支持按名称、描述、分类、状态和排序方式筛选。排序值越小，顾客端展示越靠前。"
    >
      <template #actions>
        <el-button text type="primary" :disabled="!selectedProducts.length" @click="batchEnable">批量上架</el-button>
        <el-button text type="primary" :disabled="!selectedProducts.length" @click="batchDisable">批量下架</el-button>
        <el-button text type="primary" :disabled="!selectedProducts.length" @click="openBatchCategory">批量改分类</el-button>
        <el-button text type="primary" :disabled="!selectedProducts.length" @click="batchMoveFront">选中排前</el-button>
        <el-button text type="primary" :disabled="visibleProducts.length < 2" :loading="saving" @click="reorderVisibleProducts">当前筛选重排</el-button>
      </template>
      <div class="filter-card">
        <el-input v-model="filters.keyword" clearable placeholder="搜索商品名称、描述" />
        <el-select v-model="filters.category" clearable placeholder="全部分类">
          <el-option v-for="item in categoryOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部状态">
          <el-option label="已上架" value="active" />
          <el-option label="已下架" value="inactive" />
        </el-select>
        <el-select v-model="filters.sort" placeholder="排序方式">
          <el-option label="排序值从小到大" value="sort_asc" />
          <el-option label="价格从低到高" value="price_asc" />
          <el-option label="价格从高到低" value="price_desc" />
          <el-option label="最新创建优先" value="newest" />
        </el-select>
        <el-select v-model="filters.quality" clearable placeholder="运营问题">
          <el-option label="缺少图片" value="missing_image" />
          <el-option label="缺少描述" value="missing_description" />
          <el-option label="售罄" value="sold_out" />
          <el-option label="低库存" value="low_stock" />
          <el-option label="排序靠后" value="sort_late" />
        </el-select>
      </div>

      <div class="batch-strip">
        <div>
          <span>已选 {{ selectedProducts.length }} 个商品</span>
          <strong>{{ batchHint }}</strong>
        </div>
        <el-button plain @click="resetQualityFilter">清空筛选</el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="visibleProducts"
        row-key="id"
        empty-text="暂无商品"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="44" />
        <el-table-column label="商品" min-width="280">
          <template #default="{ row }">
            <div class="product-cell">
              <div class="thumb">
                <img v-if="row.image_url" :src="row.image_url" :alt="row.name" />
                <span v-else>{{ shortName(row.name) }}</span>
              </div>
              <div>
                <strong>{{ row.name }}</strong>
                <p>{{ row.description || '暂无商品描述' }}</p>
                <div class="quality-tags">
                  <el-tag
                    v-for="tag in productQualityTags(row)"
                    :key="tag.label"
                    size="small"
                    :type="tag.type"
                    effect="light"
                  >
                    {{ tag.label }}
                  </el-tag>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" width="140">
          <template #default="{ row }">
            <el-tag effect="plain">{{ row.category || defaultCategory }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="价格" width="120" sortable>
          <template #default="{ row }"><strong>{{ formatMoney(row.price) }}</strong></template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="90" sortable />
        <el-table-column label="库存" width="190">
          <template #default="{ row }">
            <div class="stock-cell">
              <el-tag :type="stockTagType(row)" effect="light">{{ stockLabel(row) }}</el-tag>
              <div class="stock-actions">
                <el-button size="small" text type="danger" :disabled="isSoldOut(row)" @click="markSoldOut(row)">售罄</el-button>
                <el-button size="small" text type="primary" @click="addStock(row, 10)">补 10</el-button>
                <el-button size="small" text @click="clearStockLimit(row)">不限</el-button>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="顾客端展示" min-width="150">
          <template #default="{ row }">
            <div class="display-cell">
              <strong>{{ displayRank(row) }}</strong>
              <small>{{ displayHint(row) }}</small>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '已上架' : '已下架' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" @click="moveSort(row, -10)">上移</el-button>
              <el-button size="small" @click="moveSort(row, 10)">下移</el-button>
              <el-button v-if="row.status === 'active'" size="small" type="danger" plain @click="disableProduct(row)">下架</el-button>
              <el-button v-else size="small" type="success" plain @click="enableProduct(row)">上架</el-button>
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
    </DataPanel>

    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新增商品' : '编辑商品'" width="720px">
      <el-form :model="form" label-position="top">
        <div class="form-grid">
          <el-form-item label="商品名称">
            <el-input v-model="form.name" placeholder="例如：招牌羊肉串、双人套餐、基础服务" maxlength="120" />
          </el-form-item>
          <el-form-item label="商品分类">
            <el-input v-model="form.category" placeholder="例如：热销、烧烤、饮品、套餐" maxlength="80" />
          </el-form-item>
        </div>
        <div class="form-grid">
          <el-form-item label="价格（元）">
            <el-input-number v-model="form.price_yuan" :min="0" :precision="2" :step="1" class="wide-input" />
          </el-form-item>
          <el-form-item label="排序（越小越靠前）">
            <el-input-number v-model="form.sort" :min="1" :step="1" class="wide-input" />
          </el-form-item>
        </div>
        <div class="form-grid">
          <el-form-item label="库存控制">
            <el-radio-group v-model="form.stock_enabled">
              <el-radio-button :label="false">不限库存</el-radio-button>
              <el-radio-button :label="true">填写库存</el-radio-button>
            </el-radio-group>
            <small class="field-tip">不限库存适合服务类或长期供应商品；库存为 0 时顾客端会显示售罄。</small>
          </el-form-item>
          <el-form-item label="可售库存">
            <el-input-number v-model="form.stock" :min="0" :step="1" :disabled="!form.stock_enabled" class="wide-input" />
          </el-form-item>
        </div>
        <el-form-item label="商品状态">
          <el-radio-group v-model="form.status">
            <el-radio-button label="active">上架展示</el-radio-button>
            <el-radio-button label="inactive">暂时下架</el-radio-button>
          </el-radio-group>
          <small class="field-tip">下架商品不会出现在顾客扫码点单页，但会保留历史订单快照。</small>
        </el-form-item>
        <el-form-item label="商品描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="简单说明商品口味、服务内容、适用场景或推荐理由"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="商品图片">
          <div class="image-row">
            <div class="preview">
              <img v-if="form.image_url" :src="form.image_url" alt="商品图片" />
              <span v-else>暂无图片</span>
            </div>
            <div class="image-tools">
              <el-upload
                :show-file-list="false"
                accept="image/jpeg,image/png,image/webp"
                :before-upload="beforeImageUpload"
                :http-request="uploadImage"
              >
                <el-button :loading="uploading">上传图片</el-button>
              </el-upload>
              <el-input v-model="form.image_url" placeholder="也可以直接填写图片 URL" maxlength="500" />
              <small :class="{ danger: form.image_url && !isValidImageUrl(form.image_url) }">
                支持 jpg、png、webp，单张不超过 3MB。图片 URL 需以 http:// 或 https:// 开头。
              </small>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">
          {{ dialogMode === 'create' ? '创建商品' : '保存修改' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="batchCategoryVisible" title="批量修改商品分类" width="520px">
      <div class="batch-category-dialog">
        <p>已选择 {{ selectedProducts.length }} 个商品，修改后会同步影响顾客扫码页的分类展示。</p>
        <el-form label-position="top">
          <el-form-item label="目标分类">
            <el-select
              v-model="batchCategoryForm.category"
              filterable
              allow-create
              default-first-option
              placeholder="选择已有分类或输入新分类"
              class="wide-input"
            >
              <el-option v-for="item in categoryOptions" :key="item" :label="item" :value="item" />
            </el-select>
            <small class="field-tip">建议分类名称简短清晰，例如热销、套餐、饮品、服务、零售。</small>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="batchCategoryVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!selectedProducts.length" :loading="saving" @click="submitBatchCategory">
          确认修改分类
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  createMerchantStoreProduct,
  deleteMerchantStoreProduct,
  fetchMerchantStoreProducts,
  updateMerchantStoreProduct,
  uploadMerchantProductImage
} from '../../api/modules'
import DataPanel from '../../components/design/DataPanel.vue'
import MetricCard from '../../components/design/MetricCard.vue'
import PageHero from '../../components/design/PageHero.vue'

const route = useRoute()
const router = useRouter()
const storeId = Number(route.params.storeId)
const defaultCategory = '默认分类'
const products = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const dialogVisible = ref(false)
const batchCategoryVisible = ref(false)
const dialogMode = ref('create')
const editingId = ref(null)
const selectedProducts = ref([])

const filters = reactive({
  keyword: '',
  category: '',
  status: '',
  sort: 'sort_asc',
  quality: ''
})

const form = reactive({
  name: '',
  category: defaultCategory,
  price_yuan: 0,
  description: '',
  image_url: '',
  stock_enabled: false,
  stock: 0,
  sort: 100,
  status: 'active'
})

const batchCategoryForm = reactive({
  category: defaultCategory
})

const activeCount = computed(() => products.value.filter((item) => item.status === 'active').length)
const inactiveCount = computed(() => products.value.filter((item) => item.status !== 'active').length)
const missingImageCount = computed(() => products.value.filter((item) => !String(item.image_url || '').trim()).length)
const missingDescriptionCount = computed(() => products.value.filter((item) => !String(item.description || '').trim()).length)
const soldOutCount = computed(() => products.value.filter((item) => item.stock !== null && item.stock !== undefined && Number(item.stock) <= 0).length)
const lowStockCount = computed(() => products.value.filter((item) => item.stock !== null && item.stock !== undefined && Number(item.stock) > 0 && Number(item.stock) <= 5).length)
const lateSortCount = computed(() => products.value.filter((item) => Number(item.sort || 100) >= 100).length)
const menuHealthScore = computed(() => {
  if (!products.value.length) return 0
  const issueCount = missingImageCount.value + missingDescriptionCount.value + inactiveCount.value
  return Math.max(0, Math.round((1 - issueCount / Math.max(products.value.length * 3, 1)) * 100))
})
const launchScore = computed(() => {
  if (!products.value.length) return 0
  const checks = launchChecks.value.filter((item) => item.done).length
  return Math.round((checks / Math.max(launchChecks.value.length, 1)) * 100)
})
const launchTone = computed(() => {
  if (launchScore.value >= 80) return 'good'
  if (launchScore.value >= 50) return 'warning'
  return 'danger'
})
const launchBrief = computed(() => {
  if (!products.value.length) {
    return { title: '还没有商品，先建立可扫码下单的基础菜单', text: '建议先录入 8-15 个常卖商品，至少覆盖热销、套餐、饮品或服务项目。' }
  }
  if (launchScore.value >= 80) {
    return { title: '菜单可以进入扫码页验证', text: '基础信息比较完整，下一步重点看顾客点击和下单转化，再优化热销排序。' }
  }
  if (missingImageCount.value > 0) {
    return { title: '先补图片，顾客端转化会更稳', text: `还有 ${missingImageCount.value} 个商品缺图。真实扫码下单时，图片比长描述更影响点击。` }
  }
  if (soldOutCount.value > 0) {
    return { title: '先处理售罄商品，避免顾客反复点不到', text: `当前有 ${soldOutCount.value} 个商品库存为 0，可补货、下架或改为不限库存。` }
  }
  if (inactiveCount.value > 0) {
    return { title: '检查下架商品是否需要恢复', text: `当前有 ${inactiveCount.value} 个商品下架，确认是否缺货、停售或需要重新上架。` }
  }
  return { title: '继续优化描述和展示顺序', text: '把招牌商品、套餐和高毛利商品排到顾客端前面，有助于提高客单价。' }
})
const launchChecks = computed(() => [
  { label: '至少 8 个商品', done: products.value.length >= 8, hint: `${products.value.length} 个商品，菜单太少会影响选择。` },
  { label: '上架商品充足', done: activeCount.value >= Math.min(6, products.value.length || 6), hint: `${activeCount.value} 个上架，顾客只能看到上架商品。` },
  { label: '图片基本完整', done: missingImageCount.value === 0, hint: `${missingImageCount.value} 个缺图，建议优先补齐热销商品。` },
  { label: '描述基本完整', done: missingDescriptionCount.value <= Math.max(1, Math.floor(products.value.length * 0.2)), hint: `${missingDescriptionCount.value} 个缺描述，补口味/规格/推荐理由。` },
  { label: '无售罄阻塞', done: soldOutCount.value === 0, hint: `${soldOutCount.value} 个售罄商品，库存为 0 时顾客端不可点。` },
  { label: '前排排序清晰', done: lateSortCount.value < products.value.length, hint: '排序值越小越靠前，招牌和套餐建议排前。' }
])
const batchHint = computed(() => {
  if (!selectedProducts.value.length) return '可勾选多个商品后批量上架、下架或排到前面'
  const active = selectedProducts.value.filter((item) => item.status === 'active').length
  const categoryCount = new Set(selectedProducts.value.map((item) => normalizeCategory(item.category))).size
  return `已选 ${selectedProducts.value.length} 个，其中 ${active} 个上架、${selectedProducts.value.length - active} 个下架、来自 ${categoryCount} 个分类`
})
const menuHealthTip = computed(() => {
  if (!products.value.length) return '先新增商品形成菜单'
  if (menuHealthScore.value >= 85) return '菜单基础完整，适合继续做套餐和营销'
  if (missingImageCount.value > 0) return '优先补齐商品图片'
  if (soldOutCount.value > 0) return '优先处理售罄商品'
  if (missingDescriptionCount.value > 0) return '优先补齐商品描述'
  return '检查下架商品和排序'
})
const opsCards = computed(() => [
  {
    key: 'missing_image',
    label: '缺图商品',
    value: missingImageCount.value,
    hint: '图片会直接影响顾客扫码页点击率',
    tone: missingImageCount.value ? 'warning' : 'good',
    action: () => setQualityFilter('missing_image')
  },
  {
    key: 'sold_out',
    label: '售罄商品',
    value: soldOutCount.value,
    hint: '库存为 0 时顾客端不能继续加购',
    tone: soldOutCount.value ? 'danger' : 'good',
    action: () => setQualityFilter('sold_out')
  },
  {
    key: 'health',
    label: '菜单健康度',
    value: `${menuHealthScore.value}%`,
    hint: menuHealthTip.value,
    tone: menuHealthScore.value >= 85 ? 'good' : 'warning',
    action: resetQualityFilter
  },
  {
    key: 'missing_description',
    label: '缺描述商品',
    value: missingDescriptionCount.value,
    hint: '补充口味、规格、适用场景更容易成交',
    tone: missingDescriptionCount.value ? '' : 'good',
    action: () => setQualityFilter('missing_description')
  },
  {
    key: 'inactive',
    label: '已下架商品',
    value: inactiveCount.value,
    hint: '下架商品不会出现在顾客扫码页',
    tone: 'muted',
    action: () => setStatusFilter('inactive')
  },
  {
    key: 'low_stock',
    label: '低库存商品',
    value: lowStockCount.value,
    hint: '库存 1-5 份，建议及时补货或下架',
    tone: lowStockCount.value ? 'warning' : 'good',
    action: () => setQualityFilter('low_stock')
  }
])
const primaryOpsCards = computed(() => opsCards.value.slice(0, 3))
const secondaryOpsCards = computed(() => opsCards.value.slice(3))
const avgPrice = computed(() => {
  if (!products.value.length) return 0
  return Math.round(products.value.reduce((sum, item) => sum + Number(item.price || 0), 0) / products.value.length)
})
const categoryOptions = computed(() => {
  const set = new Set(products.value.map((item) => normalizeCategory(item.category)))
  return Array.from(set)
})
const categoryStats = computed(() => categoryOptions.value.map((name) => {
  const rows = products.value.filter((item) => normalizeCategory(item.category) === name)
  return {
    name,
    total: rows.length,
    active: rows.filter((item) => item.status === 'active').length
  }
}))
const visibleProducts = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase()
  const rows = products.value.filter((item) => {
    const matchKeyword = !keyword || `${item.name || ''}${item.description || ''}`.toLowerCase().includes(keyword)
    const matchCategory = !filters.category || normalizeCategory(item.category) === filters.category
    const matchStatus = !filters.status || item.status === filters.status
    const matchQuality = !filters.quality || matchesQuality(item, filters.quality)
    return matchKeyword && matchCategory && matchStatus && matchQuality
  })
  return [...rows].sort((a, b) => {
    if (filters.sort === 'price_asc') return Number(a.price || 0) - Number(b.price || 0)
    if (filters.sort === 'price_desc') return Number(b.price || 0) - Number(a.price || 0)
    if (filters.sort === 'newest') return Number(b.id || 0) - Number(a.id || 0)
    return Number(a.sort || 100) - Number(b.sort || 100)
  })
})

const normalizeCategory = (value) => value || defaultCategory
const matchesQuality = (item, quality) => {
  if (quality === 'missing_image') return !String(item.image_url || '').trim()
  if (quality === 'missing_description') return !String(item.description || '').trim()
  if (quality === 'sold_out') return item.stock !== null && item.stock !== undefined && Number(item.stock) <= 0
  if (quality === 'low_stock') return item.stock !== null && item.stock !== undefined && Number(item.stock) > 0 && Number(item.stock) <= 5
  if (quality === 'sort_late') return Number(item.sort || 100) >= 100
  return true
}
const productQualityTags = (item) => {
  const tags = []
  if (!String(item.image_url || '').trim()) tags.push({ label: '缺图', type: 'warning' })
  if (!String(item.description || '').trim()) tags.push({ label: '缺描述', type: 'info' })
  if (item.status !== 'active') tags.push({ label: '未展示', type: 'info' })
  if (item.stock !== null && item.stock !== undefined && Number(item.stock) <= 0) tags.push({ label: '售罄', type: 'danger' })
  if (item.stock !== null && item.stock !== undefined && Number(item.stock) > 0 && Number(item.stock) <= 5) tags.push({ label: '低库存', type: 'warning' })
  if (Number(item.sort || 100) >= 100) tags.push({ label: '排序靠后', type: 'warning' })
  if (!tags.length) tags.push({ label: '展示完整', type: 'success' })
  return tags
}
const displayRank = (row) => {
  if (row.status !== 'active') return '顾客不可见'
  return `第 ${visibleActiveProducts.value.findIndex((item) => item.id === row.id) + 1 || '-'} 位`
}
const visibleActiveProducts = computed(() => products.value
  .filter((item) => item.status === 'active')
  .sort((a, b) => Number(a.sort || 100) - Number(b.sort || 100)))
const displayHint = (row) => {
  if (row.status !== 'active') return '下架商品不会出现在扫码页'
  if (row.stock !== null && row.stock !== undefined && Number(row.stock) <= 0) return '已售罄，顾客不可加购'
  if (!String(row.image_url || '').trim()) return '缺图会影响点击'
  if (Number(row.sort || 100) >= 100) return '排序靠后，建议调整'
  return '扫码页正常展示'
}
const stockLabel = (row) => {
  if (row.stock === null || row.stock === undefined) return '不限库存'
  const stock = Number(row.stock)
  if (stock <= 0) return '已售罄'
  return `剩 ${stock} 份`
}
const stockTagType = (row) => {
  if (row.stock === null || row.stock === undefined) return 'info'
  const stock = Number(row.stock)
  if (stock <= 0) return 'danger'
  if (stock <= 5) return 'warning'
  return 'success'
}
const isSoldOut = (row) => row.stock !== null && row.stock !== undefined && Number(row.stock || 0) <= 0
const handleSelectionChange = (rows) => {
  selectedProducts.value = rows
}
const setQualityFilter = (quality) => {
  filters.quality = quality
  filters.status = ''
}
const setStatusFilter = (status) => {
  filters.status = status
  filters.quality = ''
}
const setCategoryFilter = (category) => {
  filters.category = category
}
const resetQualityFilter = () => {
  filters.keyword = ''
  filters.category = ''
  filters.status = ''
  filters.quality = ''
  filters.sort = 'sort_asc'
}
const openProductAI = () => {
  router.push({
    path: '/merchant/ai',
    query: {
      scenario: 'product_optimize',
      source: 'products',
      auto: '1',
      store_id: storeId,
      total: products.value.length,
      active: activeCount.value,
      categories: categoryOptions.value.length,
      missing_image: missingImageCount.value,
      missing_desc: missingDescriptionCount.value,
      sold_out: soldOutCount.value,
      low_stock: lowStockCount.value,
      health: `${menuHealthScore.value}%`,
      brief: launchBrief.value.title,
      product: products.value[0]?.name || ''
    }
  })
}
const centsToYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuanToCents = (value) => Math.round(Number(value || 0) * 100)
const formatMoney = (value) => `¥${centsToYuan(value)}`
const shortName = (value = '') => value.slice(0, 2) || '商品'
const isValidImageUrl = (value) => {
  const url = String(value || '').trim()
  if (!url) return true
  try {
    const parsed = new URL(url)
    return ['http:', 'https:'].includes(parsed.protocol)
  } catch {
    return false
  }
}

const load = async () => {
  loading.value = true
  try {
    const res = await fetchMerchantStoreProducts(storeId, { page: page.value, page_size: pageSize })
    products.value = res.data.list || []
    total.value = res.data.total || 0
    applyAIProductDraft()
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  Object.assign(form, {
    name: '',
    category: defaultCategory,
    price_yuan: 0,
    description: '',
    image_url: '',
    stock_enabled: false,
    stock: 0,
    sort: 100,
    status: 'active'
  })
  editingId.value = null
}

const openCreate = () => {
  dialogMode.value = 'create'
  resetForm()
  dialogVisible.value = true
}

const applyAIProductDraft = () => {
  if (route.query.ai_prefill !== '1' || dialogVisible.value) return
  const raw = sessionStorage.getItem('merchant_ai_product_draft')
  if (!raw) return
  try {
    const draft = JSON.parse(raw)
    dialogMode.value = 'create'
    resetForm()
    form.name = String(draft.name || '').slice(0, 80) || 'AI 商品草稿'
    form.category = normalizeCategory(draft.category || defaultCategory)
    form.description = String(draft.description || draft.ai_note || '').slice(0, 500)
    form.price_yuan = Number(draft.price_yuan || 0)
    form.sort = 20
    form.status = 'active'
    dialogVisible.value = true
    sessionStorage.removeItem('merchant_ai_product_draft')
    router.replace(`/merchant/stores/${storeId}/products`)
    ElMessage.success('已填入 AI 商品草稿，请补充价格和图片后保存')
  } catch {
    sessionStorage.removeItem('merchant_ai_product_draft')
    ElMessage.warning('AI 商品草稿读取失败，请重新生成')
  }
}

const openCustomerPreview = () => {
  window.open(`${window.location.origin}/customer/store/${storeId}`, '_blank')
}

const openEdit = (row) => {
  dialogMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    name: row.name || '',
    category: normalizeCategory(row.category),
    price_yuan: Number(row.price || 0) / 100,
    description: row.description || '',
    image_url: row.image_url || '',
    stock_enabled: row.stock !== null && row.stock !== undefined,
    stock: row.stock === null || row.stock === undefined ? 0 : Number(row.stock),
    sort: row.sort || 100,
    status: row.status || 'active'
  })
  dialogVisible.value = true
}

const buildPayload = () => ({
  store_id: storeId,
  name: form.name.trim(),
  category: form.category.trim() || defaultCategory,
  price: yuanToCents(form.price_yuan),
  description: form.description.trim(),
  image_url: form.image_url.trim(),
  stock: form.stock_enabled ? Number(form.stock || 0) : null,
  sort: Number(form.sort || 100),
  status: form.status || 'active'
})

const buildRowPayload = (row, extra = {}) => ({
  store_id: storeId,
  name: row.name,
  category: normalizeCategory(row.category),
  price: row.price,
  description: row.description || '',
  image_url: row.image_url || '',
  stock: row.stock === undefined ? null : row.stock,
  sort: row.sort || 100,
  status: row.status || 'active',
  ...extra
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
  if (!isValidImageUrl(form.image_url)) {
    ElMessage.warning('商品图片 URL 需以 http:// 或 https:// 开头')
    return false
  }
  return true
}

const submit = async () => {
  if (!validateForm()) return
  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await createMerchantStoreProduct(buildPayload())
      ElMessage.success('商品已创建')
    } else {
      await updateMerchantStoreProduct(editingId.value, buildPayload())
      ElMessage.success('商品已更新')
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '操作失败')
  } finally {
    saving.value = false
  }
}

const beforeImageUpload = (file) => {
  const allowedTypes = ['image/jpeg', 'image/png', 'image/webp']
  if (!allowedTypes.includes(file.type)) {
    ElMessage.warning('仅支持 jpg、png、webp 图片')
    return false
  }
  if (file.size > 3 * 1024 * 1024) {
    ElMessage.warning('图片不能超过 3MB')
    return false
  }
  return true
}

const uploadImage = async ({ file }) => {
  uploading.value = true
  try {
    const data = new FormData()
    data.append('file', file)
    const res = await uploadMerchantProductImage(data)
    form.image_url = res.data.url
    ElMessage.success('图片已上传')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '图片上传失败')
  } finally {
    uploading.value = false
  }
}

const updateRow = async (row, extra) => {
  await updateMerchantStoreProduct(row.id, buildRowPayload(row, extra))
  await load()
}

const moveSort = async (row, delta) => {
  await updateRow(row, { sort: Math.max(1, Number(row.sort || 100) + delta) })
  ElMessage.success('排序已更新')
}

const markSoldOut = async (row) => {
  await updateRow(row, { stock: 0 })
  ElMessage.success('已标记售罄，顾客端不可继续加购')
}

const addStock = async (row, amount) => {
  const current = row.stock === null || row.stock === undefined ? 0 : Number(row.stock || 0)
  await updateRow(row, { stock: current + amount, status: 'active' })
  ElMessage.success(`库存已增加 ${amount} 份`)
}

const clearStockLimit = async (row) => {
  await updateRow(row, { stock: null })
  ElMessage.success('已改为不限库存')
}

const disableProduct = async (row) => {
  await deleteMerchantStoreProduct(row.id)
  ElMessage.success('商品已下架')
  await load()
}

const enableProduct = async (row) => {
  await updateRow(row, { status: 'active' })
  ElMessage.success('商品已上架')
}

const batchUpdate = async (extraFactory, successText) => {
  if (!selectedProducts.value.length) return
  saving.value = true
  try {
    await Promise.all(selectedProducts.value.map((row, index) => updateMerchantStoreProduct(row.id, buildRowPayload(row, extraFactory(row, index)))))
    ElMessage.success(successText)
    selectedProducts.value = []
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '批量操作失败')
  } finally {
    saving.value = false
  }
}

const batchEnable = () => batchUpdate(() => ({ status: 'active' }), '已批量上架')
const batchDisable = () => batchUpdate(() => ({ status: 'inactive' }), '已批量下架')
const batchMoveFront = () => batchUpdate((row, index) => ({ sort: 10 + index * 10, status: 'active' }), '已将选中商品排到前面')

const openBatchCategory = () => {
  if (!selectedProducts.value.length) return
  batchCategoryForm.category = filters.category || normalizeCategory(selectedProducts.value[0]?.category)
  batchCategoryVisible.value = true
}

const submitBatchCategory = async () => {
  const category = normalizeCategory(batchCategoryForm.category).trim()
  if (!category) {
    ElMessage.warning('请填写目标分类')
    return
  }
  await batchUpdate(() => ({ category }), `已将选中商品移动到「${category}」`)
  batchCategoryVisible.value = false
  filters.category = category
}

const reorderVisibleProducts = async () => {
  const rows = visibleProducts.value
  if (rows.length < 2) return
  saving.value = true
  try {
    await Promise.all(rows.map((row, index) => updateMerchantStoreProduct(row.id, buildRowPayload(row, { sort: 10 + index * 10 }))))
    ElMessage.success('已按当前筛选顺序重排顾客端展示')
    await load()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重排商品失败')
  } finally {
    saving.value = false
  }
}

const changePage = async (nextPage) => {
  page.value = nextPage
  await load()
}

onMounted(load)
</script>

<style scoped>
.products-page {
  display: grid;
  gap: 16px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
}

.ops-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.ops-focus {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.ops-more {
  grid-column: 1 / -1;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  overflow: hidden;
  background: #ffffff;
}

.ops-grid.compact {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.ops-card {
  min-height: 110px;
  padding: 15px;
  text-align: left;
  border: 1px solid #dbeafe;
  border-radius: 20px;
  background: linear-gradient(135deg, #f8fbff, #ffffff);
  box-shadow: 0 12px 30px rgba(15, 39, 71, 0.06);
  cursor: pointer;
  transition: 0.2s ease;
}

.ops-card:hover {
  transform: translateY(-2px);
  border-color: #93c5fd;
  box-shadow: 0 18px 38px rgba(15, 39, 71, 0.1);
}

.ops-card.warning {
  border-color: #fed7aa;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.ops-card.danger {
  border-color: #fecaca;
  background: linear-gradient(135deg, #fef2f2, #ffffff);
}

.ops-card.muted {
  background: linear-gradient(135deg, #f8fafc, #ffffff);
}

.ops-card.good {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.ops-card span,
.ops-card strong,
.ops-card small {
  display: block;
}

.ops-card span {
  color: #64748b;
  font-size: 12px;
}

.ops-card strong {
  margin: 7px 0 5px;
  color: #0f2747;
  font-size: 25px;
}

.ops-card small {
  color: #64748b;
  line-height: 1.45;
}

.launch-board {
  display: grid;
  grid-template-columns: minmax(260px, 0.9fr) minmax(0, 1.6fr);
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid #dbeafe;
  border-radius: 22px;
  background: #ffffff;
  box-shadow: 0 14px 36px rgba(15, 39, 71, 0.07);
}

.launch-board.good {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.launch-board.warning {
  border-color: #fed7aa;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.launch-board.danger {
  border-color: #fecaca;
  background: linear-gradient(135deg, #fef2f2, #ffffff);
}

.launch-summary,
.launch-checks div {
  padding: 14px;
  border: 1px solid rgba(203, 213, 225, 0.82);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
}

.launch-summary span,
.launch-summary strong,
.launch-summary p,
.launch-checks span,
.launch-checks strong,
.launch-checks small {
  display: block;
}

.launch-summary span,
.launch-checks span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.launch-summary strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
  line-height: 1.25;
}

.launch-summary p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.launch-checks {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}

.launch-checks div.done {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.launch-checks strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 16px;
  line-height: 1.3;
}

.launch-checks small {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.45;
}

.category-strip {
  display: flex;
  gap: 10px;
  overflow-x: auto;
  padding: 2px 2px 8px;
}

.category-strip button {
  min-width: 128px;
  padding: 10px 12px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
  cursor: pointer;
  transition: 0.2s ease;
}

.category-strip button:hover,
.category-strip button.active {
  border-color: #2563eb;
  background: #eff6ff;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.08);
}

.category-strip span,
.category-strip strong,
.category-strip small {
  display: block;
}

.category-strip span {
  color: #64748b;
  font-size: 12px;
}

.category-strip strong {
  margin-top: 4px;
  color: #0f2747;
  font-size: 20px;
}

.category-strip small {
  margin-top: 2px;
  color: #64748b;
  font-size: 12px;
}

.filter-card {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) 160px 140px 170px 160px;
  gap: 12px;
  margin-bottom: 14px;
}

.batch-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  padding: 12px 14px;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: #f8fbff;
}

.batch-strip span,
.batch-strip strong {
  display: block;
}

.batch-strip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
}

.batch-strip strong {
  margin-top: 4px;
  color: #0f2747;
}

.product-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.product-cell p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.55;
}

.quality-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 7px;
}

.stock-cell {
  display: grid;
  gap: 5px;
}

.stock-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: wrap;
}

.display-cell {
  display: grid;
  gap: 3px;
}

.display-cell strong {
  color: #0f2747;
}

.display-cell small {
  color: #64748b;
  line-height: 1.4;
}

.thumb,
.preview {
  display: grid;
  place-items: center;
  overflow: hidden;
  background: linear-gradient(135deg, #eff6ff, #f8fafc);
  color: #2563eb;
  font-weight: 800;
  border: 1px solid #dbeafe;
}

.thumb {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  flex: 0 0 auto;
}

.thumb img,
.preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.products-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.action-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.wide-input {
  width: 100%;
}

.image-row {
  display: grid;
  grid-template-columns: 150px 1fr;
  gap: 14px;
  width: 100%;
}

.preview {
  width: 150px;
  height: 120px;
  border-radius: 18px;
}

.image-tools {
  display: grid;
  gap: 10px;
  align-content: start;
}

.image-tools small {
  color: #64748b;
  line-height: 1.55;
}

.field-tip {
  display: block;
  margin-top: 8px;
  color: #64748b;
  line-height: 1.55;
}

.batch-category-dialog p {
  margin: 0 0 16px;
  color: #64748b;
  line-height: 1.65;
}

.image-tools small.danger {
  color: #dc2626;
  font-weight: 700;
}

@media (max-width: 1180px) {
  .metric-grid,
  .ops-focus,
  .ops-grid,
  .launch-checks {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .launch-board {
    grid-template-columns: 1fr;
  }

  .filter-card {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .metric-grid,
  .ops-focus,
  .ops-grid,
  .launch-checks,
  .filter-card,
  .form-grid,
  .image-row {
    grid-template-columns: 1fr;
  }

  .batch-strip {
    align-items: flex-start;
    flex-direction: column;
  }

  .preview {
    width: 100%;
  }
}
</style>
