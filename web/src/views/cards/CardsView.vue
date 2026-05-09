<template>
  <div class="cards-stack">
    <section class="page-card cards-hero">
      <div>
        <div class="eyebrow">CARD CODES</div>
        <h2 class="page-title">卡密中心</h2>
        <p class="muted">卡密适合线下销售、渠道赠送、客服补偿和活动兑换。用户在个人中心输入卡密后，可激活对应套餐和额度。</p>
      </div>
      <el-button type="primary" @click="dialogVisible = true">批量生成卡密</el-button>
    </section>

    <section class="stat-grid">
      <div class="stat-card">
        <span>卡密总数</span>
        <strong>{{ cards.length }}</strong>
      </div>
      <div class="stat-card">
        <span>未兑换</span>
        <strong>{{ statusCount.unused }}</strong>
      </div>
      <div class="stat-card">
        <span>已兑换</span>
        <strong>{{ statusCount.used }}</strong>
      </div>
      <div class="stat-card">
        <span>已失效</span>
        <strong>{{ statusCount.expired }}</strong>
      </div>
    </section>

    <section class="page-card">
      <el-alert
        class="usage-alert"
        title="使用说明：先选择一个会员套餐批量生成卡密，再把卡密发给用户。用户兑换后会获得套餐权益，卡密状态变为已兑换。"
        type="info"
        show-icon
        :closable="false"
      />
      <el-table :data="cards">
        <el-table-column prop="batch_no" label="批次号" min-width="150" />
        <el-table-column prop="code" label="卡密" min-width="240" />
        <el-table-column label="套餐" min-width="140">
          <template #default="{ row }">{{ row.package?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="quota" label="额度" width="100" />
        <el-table-column label="兑换用户" width="120">
          <template #default="{ row }">{{ row.redeemed_by || '-' }}</template>
        </el-table-column>
        <el-table-column label="兑换时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.redeemed_at) }}</template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" title="批量生成卡密" width="520px">
      <el-form label-position="top">
        <el-form-item label="选择套餐">
          <el-select v-model="form.package_id" placeholder="请选择套餐">
            <el-option v-for="pkg in packages" :key="pkg.id" :label="packageLabel(pkg)" :value="pkg.id" />
          </el-select>
        </el-form-item>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="生成数量">
              <el-input-number v-model="form.count" :min="1" :max="500" class="full-input" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="附加额度">
              <el-input-number v-model="form.quota" :min="0" :max="999999" class="full-input" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="generate">生成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { batchCards, fetchCards, fetchPackages } from '../../api/modules'

const cards = ref([])
const packages = ref([])
const dialogVisible = ref(false)
const form = reactive({
  package_id: null,
  count: 10,
  quota: 0
})

const load = async () => {
  const [cardsRes, packagesRes] = await Promise.all([fetchCards(), fetchPackages()])
  cards.value = cardsRes.data || []
  packages.value = packagesRes.data || []
  if (!form.package_id && packages.value.length) {
    form.package_id = packages.value[0].id
  }
}

const statusCount = computed(() => cards.value.reduce((acc, item) => {
  acc[item.status] = (acc[item.status] || 0) + 1
  return acc
}, { unused: 0, used: 0, expired: 0 }))

const packageLabel = (pkg) => `${pkg.name} / ¥${(Number(pkg.price || 0) / 100).toFixed(2)}`
const statusLabel = (status) => ({ unused: '未兑换', used: '已兑换', expired: '已失效' }[status] || status || '-')
const statusType = (status) => ({ unused: 'success', used: 'info', expired: 'danger' }[status] || 'info')
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'

const generate = async () => {
  if (!form.package_id) {
    ElMessage.warning('请先选择套餐')
    return
  }
  await batchCards({ package_id: form.package_id, count: form.count, quota: form.quota })
  ElMessage.success(`已生成 ${form.count} 张卡密`)
  dialogVisible.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.cards-stack {
  display: grid;
  gap: 20px;
}

.cards-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  padding: 24px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  margin: 0;
  color: #64748b;
  line-height: 1.7;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
}

.stat-card {
  padding: 18px;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  background: #fff;
}

.stat-card span {
  display: block;
  color: #64748b;
}

.stat-card strong {
  display: block;
  margin-top: 8px;
  font-size: 28px;
}

.usage-alert {
  margin-bottom: 16px;
}

.full-input {
  width: 100%;
}
</style>
