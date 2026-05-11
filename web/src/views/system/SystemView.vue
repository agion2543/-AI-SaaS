<template>
  <div class="system-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">PRE-LAUNCH CHECK</div>
        <h2 class="page-title">上线前安全检查</h2>
        <p class="page-desc">
          集中检查管理员密码、生产环境变量、支付宝配置、AI 模型接入、数据库备份、限流、跨域和日志状态，并给出可执行的修复步骤。
        </p>
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
        <el-button text type="primary" @click="openGuide(item)">查看处理步骤</el-button>
      </div>
    </section>

    <section class="page-card ai-card">
      <div class="toolbar">
        <div>
          <div class="eyebrow">AI COMMERCIALIZATION</div>
          <h2 class="page-title">AI 落地建议</h2>
          <p class="page-desc">当前项目的差异化重点建议放在“AI 帮商家多赚钱”，而不是泛泛聊天。</p>
        </div>
      </div>
      <div class="ai-grid">
        <div>
          <strong>推荐模型接入顺序</strong>
          <p>先接 DeepSeek 或通义千问的 OpenAI 兼容接口，成本低、国内网络友好；后续再增加多模型切换。</p>
        </div>
        <div>
          <strong>优先售卖能力</strong>
          <p>AI 经营分析、AI 短视频脚本、AI 裂变海报文案、AI 一键生成优惠券，最容易让商家感知价值。</p>
        </div>
        <div>
          <strong>成本控制</strong>
          <p>按商家订阅等级限制每日生成次数，缓存相同分析结果，避免无限调用模型导致成本失控。</p>
        </div>
      </div>
      <div class="command-list">
        <code>AI_ENABLED=true</code>
        <code>AI_PROVIDER=deepseek</code>
        <code>AI_BASE_URL=https://api.deepseek.com/v1</code>
        <code>AI_MODEL=deepseek-chat</code>
      </div>
    </section>

    <section class="page-card wizard-card">
      <div class="toolbar">
        <div>
          <h2 class="page-title">生产配置修复向导</h2>
          <p class="page-desc">按优先级处理危险项：先改默认密码和密钥，再处理生产域名、支付回调、AI 配置、备份与日志。</p>
        </div>
      </div>
      <el-collapse accordion>
        <el-collapse-item v-for="item in checks" :key="`guide-${item.key}`" :name="item.key">
          <template #title>
            <div class="guide-title">
              <el-tag size="small" :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
              <strong>{{ item.title }}</strong>
              <span>{{ guideFor(item.key).summary }}</span>
            </div>
          </template>
          <ol class="guide-steps">
            <li v-for="step in guideFor(item.key).steps" :key="step">{{ step }}</li>
          </ol>
          <div v-if="guideFor(item.key).commands?.length" class="command-list">
            <code v-for="command in guideFor(item.key).commands" :key="command">{{ command }}</code>
          </div>
        </el-collapse-item>
      </el-collapse>
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

    <el-dialog v-model="guideDialogVisible" :title="activeGuide?.title || '处理步骤'" width="680px">
      <p class="dialog-desc">{{ activeGuide?.description }}</p>
      <ol class="guide-steps">
        <li v-for="step in activeGuide?.steps || []" :key="step">{{ step }}</li>
      </ol>
      <div v-if="activeGuide?.commands?.length" class="command-list">
        <code v-for="command in activeGuide.commands" :key="command">{{ command }}</code>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchConfigs, fetchSecurityCheck, saveConfigs } from '../../api/modules'

const checking = ref(false)
const guideDialogVisible = ref(false)
const activeGuide = ref(null)
const report = ref({ overall_status: 'warning', items: [] })
const form = reactive({
  site_name: '本地生活商家 AI 运营 SaaS',
  payment_gateway: '{"provider":"alipay","mode":"platform_collect_manual_settlement"}',
  filing_info: '请填写真实备案号',
  notice: '欢迎使用本地生活商家 AI 运营 SaaS。'
})

const repairGuides = {
  admin_password: {
    summary: '修改默认管理员密码，避免后台被扫描登录。',
    steps: ['登录平台后台。', '把 admin 默认密码改成 12 位以上强密码。', '退出后重新登录，确认旧密码不可用。']
  },
  app_env: {
    summary: '切换 APP_ENV，关闭开发测试能力。',
    steps: ['复制 .env.production.example 为生产 .env。', '设置 APP_ENV=production。', '重启后端并重新检查。'],
    commands: ['APP_ENV=production']
  },
  secrets: {
    summary: '生成强 JWT/AES 密钥，禁止使用默认值。',
    steps: ['JWT_SECRET 使用至少 32 位随机字符串。', 'AES_SECRET 必须为 32 个字符。', '不要把生产密钥提交到 GitHub。'],
    commands: [
      'powershell -Command "[guid]::NewGuid().ToString(\\"N\\") + [guid]::NewGuid().ToString(\\"N\\")"',
      'powershell -Command "[guid]::NewGuid().ToString(\\"N\\").Substring(0,32)"'
    ]
  },
  database: {
    summary: '生产数据库使用独立账号和最小权限。',
    steps: ['不要用 root 连接业务库。', '创建独立 MySQL 用户。', '数据库不要暴露公网。', '配置每日自动备份。']
  },
  alipay: {
    summary: '切换正式支付宝配置和 HTTPS 回调。',
    steps: ['准备支付宝正式应用资质。', '填写正式 APPID、私钥和支付宝公钥。', '设置 ALIPAY_SANDBOX=false。', '使用公网 HTTPS 回调地址。'],
    commands: ['ALIPAY_SANDBOX=false', 'ALIPAY_NOTIFY_URL=https://your-domain.com/api/v1/payments/callback/alipay']
  },
  ai: {
    summary: '接入国内兼容 OpenAI 协议的大模型。',
    steps: ['优先选择 DeepSeek、通义千问或智谱。', '申请 API Key 并确认余额。', '填写 AI_BASE_URL、AI_API_KEY、AI_MODEL。', '重启后端，在商家端 AI 页面生成一次短视频脚本测试。'],
    commands: ['AI_ENABLED=true', 'AI_PROVIDER=deepseek', 'AI_BASE_URL=https://api.deepseek.com/v1', 'AI_MODEL=deepseek-chat']
  },
  backup: {
    summary: '配置数据库自动备份与恢复演练。',
    steps: ['Windows 使用 scripts/backup-mysql.ps1。', 'Linux 使用 scripts/backup-mysql.sh。', '加入计划任务或 cron。', '每月做一次恢复演练。'],
    commands: ['powershell.exe -ExecutionPolicy Bypass -File scripts/backup-mysql.ps1']
  },
  rate_limit: {
    summary: '开启接口限流，保护登录、短信和支付接口。',
    steps: ['设置 RATE_LIMIT_PER_MINUTE。', '初期可设置 120-300。', '上线后观察 429 响应和用户反馈。'],
    commands: ['RATE_LIMIT_PER_MINUTE=180']
  },
  cors: {
    summary: '只允许正式前端域名访问 API。',
    steps: ['把 FRONTEND_URL 改为正式 HTTPS 域名。', '不要使用 * 作为跨域来源。', '拆分域名时逐个加入可信来源。'],
    commands: ['FRONTEND_URL=https://your-domain.com']
  },
  logging: {
    summary: '保留请求日志、错误日志和关键操作审计。',
    steps: ['保留登录、支付、退款、开通订阅、冻结商家等操作审计。', '配置日志轮转。', '关键错误接入告警。']
  }
}

const checks = computed(() => report.value.items || [])
const overallType = computed(() => statusType(report.value.overall_status))
const overallLabel = computed(() => ({ pass: '可上线', warning: '需复核', danger: '暂不建议上线' })[report.value.overall_status] || '需复核')

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

const guideFor = (key) => repairGuides[key] || {
  summary: '按提示完成配置修复。',
  steps: ['查看当前检查项说明。', '按建议修复配置。', '重启后端并重新检查。']
}

const openGuide = (item) => {
  const guide = guideFor(item.key)
  activeGuide.value = {
    title: item.title,
    description: item.suggestion,
    ...guide
  }
  guideDialogVisible.value = true
}

const statusType = (status) => ({ pass: 'success', warning: 'warning', danger: 'danger' })[status] || 'info'
const statusLabel = (status) => ({ pass: '通过', warning: '警告', danger: '危险' })[status] || '未知'

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

.page-desc,
.dialog-desc {
  margin: 6px 0 0;
  color: var(--muted);
  line-height: 1.7;
}

.check-grid,
.ai-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.check-card,
.ai-grid div {
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

.check-head,
.guide-title {
  display: flex;
  gap: 10px;
  align-items: center;
}

.check-head {
  justify-content: space-between;
}

.check-card p,
.ai-grid p {
  margin: 12px 0 8px;
  color: #334155;
  line-height: 1.7;
}

.check-card span,
.guide-title span {
  color: #64748b;
  line-height: 1.6;
}

.ai-card {
  border: 1px solid #bbf7d0;
  background: radial-gradient(circle at top right, rgba(34, 197, 94, 0.12), transparent 32%), #fff;
}

.ai-grid {
  margin-top: 16px;
}

.wizard-card {
  padding: 18px;
}

.guide-title {
  width: 100%;
}

.guide-steps {
  margin: 12px 0 0;
  padding-left: 22px;
  color: #334155;
  line-height: 1.9;
}

.command-list {
  display: grid;
  gap: 8px;
  margin-top: 14px;
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
  white-space: pre-wrap;
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

  .guide-title {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
