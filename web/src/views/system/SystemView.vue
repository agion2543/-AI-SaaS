<template>
  <div class="system-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">PRE-LAUNCH CHECK</div>
        <h2 class="page-title">上线前安全检查</h2>
        <p class="page-desc">集中检查管理员密码、生产环境变量、支付宝、数据库备份、限流、跨域和日志状态。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="overallType">{{ overallLabel }}</el-tag>
        <el-button type="primary" :loading="checking" @click="loadSecurityCheck">重新检查</el-button>
      </div>
    </section>

    <section class="check-grid">
      <div v-for="item in checks" :key="item.key" class="check-card" :class="item.status">
        <div class="check-head">
          <strong>{{ item.title }}</strong>
          <el-tag :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
        </div>
        <p>{{ item.description }}</p>
        <span>{{ item.suggestion }}</span>
      </div>
    </section>

    <section class="page-card ops-card">
      <div>
        <h3>数据库备份脚本入口</h3>
        <p>上线后建议把备份脚本加入 Windows 计划任务或 Linux cron，并定期做恢复演练。</p>
      </div>
      <div class="script-list">
        <code>scripts/backup-mysql.ps1</code>
        <code>scripts/backup-mysql.sh</code>
      </div>
    </section>

    <section class="page-card">
      <div class="toolbar">
        <div>
          <h2 class="page-title">系统配置</h2>
          <p class="page-desc">站点名称、支付说明、备案信息和公告设置。</p>
        </div>
        <el-button type="primary" @click="save">保存配置</el-button>
      </div>

      <el-form :model="form" label-width="120px" class="config-form">
        <el-form-item label="站点名称">
          <el-input v-model="form.site_name" />
        </el-form-item>
        <el-form-item label="支付接口说明">
          <el-input v-model="form.payment_gateway" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item label="备案信息">
          <el-input v-model="form.filing_info" />
        </el-form-item>
        <el-form-item label="公告设置">
          <el-input v-model="form.notice" type="textarea" :rows="4" />
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchConfigs, fetchSecurityCheck, saveConfigs } from '../../api/modules'

const checking = ref(false)
const report = ref({ overall_status: 'warning', items: [] })
const form = reactive({
  site_name: '本地生活商家 AI 运营 SaaS',
  payment_gateway: '{"provider":"alipay","mode":"platform_collect_manual_settlement"}',
  filing_info: '请填写真实备案号',
  notice: '欢迎使用本地生活商家 AI 运营 SaaS。'
})

const checks = computed(() => report.value.items || [])
const overallType = computed(() => statusType(report.value.overall_status))
const overallLabel = computed(() => ({
  pass: '可上线',
  warning: '需复核',
  danger: '暂不建议上线'
})[report.value.overall_status] || '需复核')

const loadConfigs = async () => {
  const res = await fetchConfigs()
  ;(res.data || []).forEach((item) => {
    if (item.config_key in form) {
      form[item.config_key] = item.config_value
    }
  })
}

const loadSecurityCheck = async () => {
  checking.value = true
  try {
    const res = await fetchSecurityCheck()
    report.value = res.data || { overall_status: 'warning', items: [] }
  } finally {
    checking.value = false
  }
}

const save = async () => {
  await saveConfigs(form)
  ElMessage.success('配置已保存')
}

const statusType = (status) => ({
  pass: 'success',
  warning: 'warning',
  danger: 'danger'
})[status] || 'info'

const statusLabel = (status) => ({
  pass: '通过',
  warning: '警告',
  danger: '危险'
})[status] || '未知'

onMounted(async () => {
  await Promise.all([loadConfigs(), loadSecurityCheck()])
})
</script>

<style scoped>
.system-stack {
  display: grid;
  gap: 18px;
}

.hero-card,
.toolbar,
.hero-actions {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.hero-card {
  padding: 22px;
}

.eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.page-desc {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.7;
}

.check-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.check-card {
  padding: 18px;
  border-radius: 18px;
  border: 1px solid #dbeafe;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 16px 38px rgba(15, 23, 42, 0.08);
}

.check-card.pass {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #f0fdf4, #ffffff);
}

.check-card.warning {
  border-color: #fde68a;
  background: linear-gradient(135deg, #fffbeb, #ffffff);
}

.check-card.danger {
  border-color: #fecaca;
  background: linear-gradient(135deg, #fff1f2, #ffffff);
}

.check-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
}

.check-card p {
  margin: 12px 0 8px;
  color: #334155;
  line-height: 1.7;
}

.check-card span {
  color: #64748b;
  line-height: 1.6;
}

.ops-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 18px;
}

.ops-card p {
  margin: 6px 0 0;
  color: var(--muted);
}

.script-list {
  display: grid;
  gap: 8px;
}

code {
  padding: 8px 12px;
  border-radius: 10px;
  background: #0f172a;
  color: #dbeafe;
}

.config-form {
  margin-top: 16px;
  max-width: 820px;
}

@media (max-width: 768px) {
  .hero-card,
  .toolbar,
  .ops-card {
    display: grid;
  }
}
</style>
