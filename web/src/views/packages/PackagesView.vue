<template>
  <div class="plans-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">SAAS PLANS</div>
        <h2 class="page-title">平台套餐</h2>
        <p class="muted">这里配置平台卖给商家的 SaaS 订阅套餐，例如月付、年付、试用、企业版。顾客不在这里购买会员。</p>
      </div>
      <el-button type="primary" @click="openDialog()">新增平台套餐</el-button>
    </section>

    <section class="page-card module-note">
      <div><strong>当前作用</strong><span>决定商家订阅页展示哪些套餐，以及支付金额和服务周期。</span></div>
      <div><strong>运营建议</strong><span>节假日前可设置试用套餐；旺季推年付；淡季推月付折扣或赠送 AI 分析权益。</span></div>
      <div><strong>后续扩展</strong><span>可以增加功能权益字段，例如商品数量、AI 次数、门店数量、员工账号数。</span></div>
    </section>

    <section class="page-card">
      <el-table :data="plans">
        <el-table-column prop="name" label="套餐名称" min-width="160" />
        <el-table-column label="价格" width="140">
          <template #default="{ row }">¥{{ formatYuan(row.price_cents) }}</template>
        </el-table-column>
        <el-table-column label="服务周期" width="140">
          <template #default="{ row }">{{ row.duration_days }} 天</template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="100" />
        <el-table-column prop="created_at" label="创建时间" min-width="170" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openDialog(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑平台套餐' : '新增平台套餐'" width="520px">
      <el-form label-position="top">
        <el-form-item label="套餐名称">
          <el-input v-model="form.name" placeholder="例如：月付、年付、企业版" />
        </el-form-item>
        <el-row :gutter="14">
          <el-col :span="12">
            <el-form-item label="价格（元）">
              <el-input-number v-model="form.price_yuan" :min="0" :precision="2" class="full-input" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="服务周期（天）">
              <el-input-number v-model="form.duration_days" :min="1" :max="3650" class="full-input" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" class="full-input" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createMerchantPlan, fetchMerchantPlans, updateMerchantPlan } from '../../api/modules'

const plans = ref([])
const dialogVisible = ref(false)
const form = reactive({
  id: 0,
  name: '',
  price_yuan: 0,
  duration_days: 30,
  sort: 0
})

const load = async () => {
  const res = await fetchMerchantPlans()
  plans.value = res.data || []
}

const formatYuan = (value) => (Number(value || 0) / 100).toFixed(2)
const yuanToFen = (value) => Math.round(Number(value || 0) * 100)

const openDialog = (row) => {
  Object.assign(form, {
    id: row?.id || 0,
    name: row?.name || '',
    price_yuan: Number(row?.price_cents || 0) / 100,
    duration_days: row?.duration_days || 30,
    sort: row?.sort || 0
  })
  dialogVisible.value = true
}

const submit = async () => {
  const payload = {
    name: form.name.trim(),
    price_cents: yuanToFen(form.price_yuan),
    duration_days: form.duration_days,
    sort: form.sort
  }
  if (form.id) {
    await updateMerchantPlan(form.id, payload)
  } else {
    await createMerchantPlan(payload)
  }
  ElMessage.success('平台套餐已保存')
  dialogVisible.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.plans-stack { display: grid; gap: 20px; }
.hero-card { display: flex; justify-content: space-between; gap: 20px; align-items: flex-start; padding: 24px; }
.eyebrow { color: #2563eb; font-size: 12px; font-weight: 900; letter-spacing: 0.16em; }
.muted { color: #64748b; margin: 0; line-height: 1.7; }
.module-note { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; padding: 18px; }
.module-note div { padding: 14px; border-radius: 14px; background: #f8fbff; border: 1px solid #e5edf9; }
.module-note strong, .module-note span { display: block; }
.module-note span { margin-top: 6px; color: var(--muted); font-size: 13px; }
.full-input { width: 100%; }
</style>
