<template>
  <div class="system-page">
    <PageHero
      eyebrow="OPS HEALTH CENTER"
      title="运营健康中心"
      description="每天查看交易、退款、结算、商家运营、部署配置和 AI 可用性风险；这里只做摘要和处理入口，不重复商家运营和订单列表。"
      compact
    >
      <template #actions>
        <el-tag size="large" :type="overallType">{{ overallLabel }}</el-tag>
        <el-button type="primary" :loading="checking" @click="loadSecurityCheck">重新检查</el-button>
      </template>
    </PageHero>

    <section class="health-summary">
      <article :class="['health-overall', report.overall_status]">
        <span>当前系统状态</span>
        <strong>{{ overallLabel }}</strong>
        <p>{{ overallDescription }}</p>
      </article>
      <article>
        <span>风险板块</span>
        <strong>{{ healthSections.length }}</strong>
        <p>覆盖交易、结算、商家、商品和部署。</p>
      </article>
      <article>
        <span>需优先处理</span>
        <strong>{{ dangerSectionCount }}</strong>
        <p>存在危险状态的运行风险板块。</p>
      </article>
      <article>
        <span>配置检查</span>
        <strong>{{ checks.length }}</strong>
        <p>保留上线与生产配置检查。</p>
      </article>
    </section>

    <section class="patrol-lanes">
      <article v-for="lane in patrolLanes" :key="lane.key" :class="lane.status">
        <div>
          <span>{{ lane.label }}</span>
          <strong>{{ lane.title }}</strong>
          <p>{{ lane.summary }}</p>
        </div>
        <el-button size="small" text type="primary" @click="goAction(lane.actionPath)">
          {{ lane.actionText }}
        </el-button>
      </article>
    </section>

    <DataPanel
      title="今日优先处理"
      description="把运行风险和配置风险合并成一张待处理清单，平台每天打开先看这里。"
      eyebrow="TODAY ACTIONS"
    >
      <div class="priority-list" v-if="priorityItems.length">
        <article v-for="item in priorityItems" :key="`${item.source}-${item.key}`" :class="item.status">
          <div>
            <el-tag :type="statusType(item.status)">{{ item.group }}</el-tag>
            <strong>{{ item.title }}</strong>
            <span>{{ item.summary }}</span>
          </div>
          <el-button size="small" type="primary" plain @click="handlePriority(item)">{{ item.actionText }}</el-button>
        </article>
      </div>
      <el-empty v-else description="当前没有需要优先处理的风险项" />
    </DataPanel>

    <DataPanel
      title="运行风险巡检"
      description="只展示摘要、严重程度和跳转入口；具体商家、订单、结算仍回到对应业务页面处理。"
      eyebrow="DAILY OPS PATROL"
    >
      <div class="health-grid">
        <article v-for="section in healthSections" :key="section.key" class="health-card" :class="section.status">
          <div class="check-head">
            <strong>{{ section.title }}</strong>
            <el-tag :type="statusType(section.status)">{{ statusLabel(section.status) }}</el-tag>
          </div>
          <p>{{ section.summary }}</p>
          <span>{{ section.suggestion }}</span>
          <div class="health-metrics">
            <div v-for="metric in section.metrics || []" :key="`${section.key}-${metric.label}`" :class="metric.status">
              <small>{{ metric.label }}</small>
              <strong>{{ metric.value }}</strong>
              <em>{{ metric.hint }}</em>
            </div>
          </div>
          <el-button v-if="section.action_path" text type="primary" @click="goAction(section.action_path)">
            {{ section.action_text || '去处理' }}
          </el-button>
        </article>
      </div>
    </DataPanel>

    <DataPanel
      title="上线与系统配置检查"
      description="这部分保留为生产配置、密钥、支付、备份、限流、跨域和日志审计检查。"
      eyebrow="CONFIG CHECK"
    >
      <div class="check-grid">
      <article v-for="item in checks" :key="item.key" class="check-card" :class="item.status">
        <div class="check-head">
          <strong>{{ item.title }}</strong>
          <el-tag :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
        </div>
        <p>{{ item.description }}</p>
        <span>{{ item.suggestion }}</span>
        <el-button text type="primary" @click="openGuide(item)">查看处理步骤</el-button>
      </article>
      </div>
    </DataPanel>

    <DataPanel
      title="后续优化方向"
      description="围绕真实商用继续推进：支付收款边界、部署迁移、顾客下单链路和商品管理补齐。"
      eyebrow="NEXT PRIORITIES"
    >
      <div class="ai-grid">
        <ActionCard
          label="支付收款"
          title="先明确 MVP 收款边界"
          description="建议平台收 SaaS 订阅费，顾客点单款优先走商家自有收款，平台做记录、对账和运营工具。"
          tone="blue"
        />
        <ActionCard
          label="部署迁移"
          title="整理生产升级路径"
          description="按 docs/production-runbook.md 执行首次部署、升级迁移、备份恢复和回滚流程，避免临时上线。"
          tone="green"
        />
        <ActionCard
          label="商用补齐"
          title="复核顾客链路和商品管理"
          description="继续从真实顾客扫码下单、支付状态、商品排序、库存售罄和图片稳定性补齐商用细节。"
          tone="slate"
        />
      </div>
    </DataPanel>

    <DataPanel
      title="生产配置修复向导"
      description="按优先级处理危险项：先改默认密码和密钥，再处理生产域名、支付回调、AI 配置、备份与日志。"
      eyebrow="FIX WIZARD"
    >
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
    </DataPanel>

    <DataPanel title="数据库备份脚本入口" description="上线后建议加入 Windows 计划任务或 Linux cron，并定期做恢复演练。" eyebrow="BACKUP">
      <div class="script-grid">
        <ActionCard
          label="Runbook"
          title="生产上线手册"
          description="把首次部署、升级发布、数据库迁移、回滚和每日巡检放在同一份手册里，适合个人开发低成本执行。"
          tone="slate"
        >
          <template #actions><code>docs/production-runbook.md</code></template>
        </ActionCard>
        <ActionCard
          label="Windows"
          title="PowerShell 备份"
          description="适合本机或 Windows Server 部署，配合计划任务每日执行。"
          tone="blue"
        >
          <template #actions><code>scripts/backup-mysql.ps1</code></template>
        </ActionCard>
        <ActionCard
          label="Linux"
          title="Shell 备份"
          description="适合云服务器部署，配合 cron 定时备份并保留最近多天数据。"
          tone="green"
        >
          <template #actions><code>scripts/backup-mysql.sh</code></template>
        </ActionCard>
      </div>
    </DataPanel>

    <DataPanel title="AI 接入配置中心" description="用于上线前确认国内模型供应商、环境变量和可用状态；API Key 仍建议放服务器环境变量，不在后台明文保存。" eyebrow="AI PROVIDER">
      <template #actions>
        <el-button plain :loading="aiChecking" @click="loadAIStatus">刷新 AI 状态</el-button>
        <el-button type="primary" plain @click="copyText(aiStatus.env_example || '')">复制环境变量</el-button>
      </template>
      <section class="ai-config-center">
        <article :class="['ai-status-card', aiStatus.status]">
          <span>当前 AI 状态</span>
          <strong>{{ aiStatus.ready ? '已可调用' : aiStatus.enabled ? '配置不完整' : '模板兜底' }}</strong>
          <p>{{ aiStatus.summary || '正在读取 AI 配置状态。' }}</p>
          <div class="ai-status-meta">
            <el-tag :type="statusType(aiStatus.status)">{{ aiStatus.provider || 'template' }}</el-tag>
            <el-tag effect="plain">{{ aiStatus.model || '未配置模型' }}</el-tag>
            <el-tag :type="aiStatus.has_api_key ? 'success' : 'warning'">{{ aiStatus.has_api_key ? 'Key 已配置' : 'Key 未配置' }}</el-tag>
          </div>
        </article>
        <article class="ai-env-card">
          <span>推荐生产配置</span>
          <pre>{{ aiStatus.env_example || 'AI_ENABLED=true\nAI_PROVIDER=deepseek\nAI_BASE_URL=https://api.deepseek.com/v1\nAI_API_KEY=replace-with-ai-api-key\nAI_MODEL=deepseek-chat\nAI_TIMEOUT_SECONDS=20' }}</pre>
          <p>{{ aiStatus.suggestion || '开启前先确认 API Key、额度、超时和计费预警。' }}</p>
        </article>
      </section>
      <section class="ai-usage-panel">
        <div class="ai-usage-head">
          <div>
            <span>AI COST & QUOTA</span>
            <strong>今日调用与成本观察</strong>
            <p>{{ aiUsage.suggestion || '记录每次 AI 生成，方便后续做套餐额度、缓存和成本预警。' }}</p>
          </div>
          <el-button text type="primary" :loading="aiChecking" @click="loadAIStatus">刷新</el-button>
        </div>
        <div class="ai-usage-grid">
          <article>
            <span>今日调用</span>
            <strong>{{ aiUsage.today_total || 0 }}</strong>
            <small>{{ aiUsage.active_merchants || 0 }} 个商家使用</small>
          </article>
          <article>
            <span>成功 / 失败</span>
            <strong>{{ aiUsage.today_success || 0 }} / {{ aiUsage.today_failed || 0 }}</strong>
            <small>失败率 {{ aiUsage.failure_rate || 0 }}%</small>
          </article>
          <article>
            <span>模板兜底</span>
            <strong>{{ aiUsage.today_fallback || 0 }}</strong>
            <small>未走真实模型的调用</small>
          </article>
          <article>
            <span>平均耗时</span>
            <strong>{{ aiUsage.average_latency_ms || 0 }}ms</strong>
            <small>用于观察供应商响应速度</small>
          </article>
        </div>
        <div class="ai-scenario-list">
          <span>高频场景</span>
          <el-tag v-for="item in aiUsage.top_scenarios || []" :key="item.scenario" effect="plain">
            {{ item.scenario || 'unknown' }} · {{ item.count }}
          </el-tag>
          <small v-if="!(aiUsage.top_scenarios || []).length">今日暂无场景数据</small>
        </div>
      </section>
      <div class="ai-provider-grid">
        <article v-for="item in aiStatus.presets || []" :key="item.provider">
          <span>{{ item.provider }}</span>
          <strong>{{ item.name }}</strong>
          <p>{{ item.description }}</p>
          <code>{{ item.base_url }}</code>
          <small>{{ item.model }}</small>
        </article>
      </div>
    </DataPanel>

    <DataPanel title="系统配置" description="站点名称、收款策略说明、备案信息和公告设置。" eyebrow="SYSTEM CONFIG">
      <template #actions>
        <el-button type="primary" @click="save">保存配置</el-button>
      </template>
      <el-form :model="form" label-width="120px" class="config-form">
        <el-form-item label="站点名称">
          <el-input v-model="form.site_name" />
        </el-form-item>
        <el-alert
          class="config-alert"
          type="info"
          show-icon
          :closable="false"
          title="当前推荐：平台只收商家 SaaS 订阅费；顾客点单款优先进入商家自有收款账户，平台做订单记录、对账和巡检。"
        />
        <el-form-item label="收款策略说明">
          <el-input
            v-model="form.payment_gateway"
            type="textarea"
            :rows="7"
            placeholder="建议记录：平台收 SaaS 订阅费，顾客点单款走商家自有收款账户，平台只做订单记录、对账和运营巡检。"
          />
        </el-form-item>
        <el-form-item label="备案信息">
          <el-input v-model="form.filing_info" />
        </el-form-item>
        <el-form-item label="平台支付宝收款码">
          <div class="config-qr-line">
            <el-input v-model="form.platform_alipay_qr_code" placeholder="用于商家订阅付费页展示，支持图片链接或支付链接" />
            <el-upload :show-file-list="false" :http-request="(options) => uploadPlatformQr(options, 'alipay')" accept="image/*">
              <el-button :loading="uploadingQr === 'alipay'">上传收款码</el-button>
            </el-upload>
          </div>
        </el-form-item>
        <el-form-item label="平台微信收款码">
          <div class="config-qr-line">
            <el-input v-model="form.platform_wechat_qr_code" placeholder="用于商家订阅付费页展示，支持图片链接或支付链接" />
            <el-upload :show-file-list="false" :http-request="(options) => uploadPlatformQr(options, 'wechat')" accept="image/*">
              <el-button :loading="uploadingQr === 'wechat'">上传收款码</el-button>
            </el-upload>
          </div>
        </el-form-item>
        <el-form-item label="订阅收款状态">
          <div class="platform-qr-status" :class="{ ready: platformQrReady }">
            <el-tag :type="platformQrReady ? 'success' : 'warning'">{{ platformQrReady ? '已配置可收款' : '建议至少配置一个收款码' }}</el-tag>
            <span>商家订阅付费页会展示已配置的支付宝 / 微信收款码；平台确认到账后开通或续期订阅。</span>
          </div>
        </el-form-item>
        <el-form-item label="订阅收款说明">
          <el-input
            v-model="form.platform_subscription_note"
            type="textarea"
            :rows="3"
            placeholder="例如：付款后请点击我已付款，平台确认到账后开通套餐。"
          />
        </el-form-item>
        <el-form-item label="公告设置">
          <el-input v-model="form.notice" type="textarea" :rows="4" />
        </el-form-item>
      </el-form>
    </DataPanel>

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
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchAdminAIConfigStatus, fetchAdminAIUsageOverview, fetchConfigs, fetchSecurityCheck, saveConfigs, uploadPlatformPaymentQRCode } from '../../api/modules'
import ActionCard from '../../components/design/ActionCard.vue'
import DataPanel from '../../components/design/DataPanel.vue'
import PageHero from '../../components/design/PageHero.vue'

const checking = ref(false)
const aiChecking = ref(false)
const uploadingQr = ref('')
const router = useRouter()
const guideDialogVisible = ref(false)
const activeGuide = ref(null)
const report = ref({ overall_status: 'warning', items: [] })
const aiStatus = ref({})
const aiUsage = ref({})
const form = reactive({
  site_name: '本地生活商家 AI 运营 SaaS',
  payment_gateway: JSON.stringify({
    strategy: 'merchant_direct_mvp',
    platform_subscription: 'platform_account',
    customer_store_order: 'merchant_own_account',
    settlement_role: 'reconciliation_record_only',
    refund_role: 'record_first_channel_manual_or_future_api',
    split_payment: 'future_service_provider_required'
  }, null, 2),
  platform_alipay_qr_code: '',
  platform_wechat_qr_code: '',
  platform_subscription_note: '付款后请点击“我已付款”，平台确认到账后会开通或续期订阅。',
  filing_info: '请填写真实备案号',
  notice: '欢迎使用本地生活商家 AI 运营 SaaS。'
})

const repairGuides = {
  admin_password: {
    summary: '修改初始化管理员密码，避免后台被扫描登录。',
    steps: ['使用初始化账号登录平台后台。', '把管理员密码改成 12 位以上强密码。', '退出后重新登录，确认旧密码和初始化密码不可用。', '生产环境不要长期保留 INITIAL_ADMIN_PASSWORD 的默认值。'],
    commands: ['INITIAL_ADMIN_USERNAME=admin', 'INITIAL_ADMIN_PASSWORD=请替换为独立强密码']
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
    steps: ['先阅读 docs/production-runbook.md，确认首次部署、升级、回滚和巡检步骤。', 'Windows 使用 scripts/backup-mysql.ps1。', 'Linux 使用 scripts/backup-mysql.sh。', '加入计划任务或 cron。', '每月做一次恢复演练。'],
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
  bootstrap_safety: {
    summary: '生产环境关闭开发期自动激活逻辑。',
    steps: ['确认 .env.production 中 APP_ENV=production。', '重启后端服务。', '注册一个测试商家，确认不会绕过平台审核自动变为 active。'],
    commands: ['APP_ENV=production']
  },
  logging: {
    summary: '保留请求日志、错误日志和关键操作审计。',
    steps: ['保留登录、支付、退款、开通订阅、冻结商家等操作审计。', '配置日志轮转。', '关键错误接入告警。']
  }
}

const checks = computed(() => report.value.items || [])
const healthSections = computed(() => report.value.health_sections || [])
const dangerSectionCount = computed(() => healthSections.value.filter((item) => item.status === 'danger').length)
const sectionMap = computed(() => Object.fromEntries(healthSections.value.map((item) => [item.key, item])))
const configRiskCount = computed(() => checks.value.filter((item) => ['danger', 'warning'].includes(item.status)).length)
const laneStatus = (...items) => {
  const validItems = items.flat().filter(Boolean)
  if (validItems.some((item) => item.status === 'danger')) return 'danger'
  if (validItems.some((item) => item.status === 'warning')) return 'warning'
  return 'pass'
}
const patrolLanes = computed(() => {
  const transaction = sectionMap.value.transaction
  const merchantOps = sectionMap.value.merchant_ops
  const productReadiness = sectionMap.value.product_readiness
  const paymentRefund = sectionMap.value.payment_refund
  const settlement = sectionMap.value.settlement
  const deployment = sectionMap.value.deployment
  return [
    {
      key: 'transaction',
      label: '交易闭环',
      title: laneTitle(transaction, '看订单是否卡住'),
      summary: transaction?.summary || '聚合待支付、支付失败、待确认收款、待接单和履约超时。',
      status: laneStatus(transaction),
      actionText: '看订单异常',
      actionPath: transaction?.action_path || '/admin/orders?exception=all'
    },
    {
      key: 'merchant',
      label: '商家运营',
      title: laneTitle(merchantOps, '看商家是否可经营'),
      summary: merchantOps?.summary || '聚合冻结、审核、订阅到期、跟进事项和门店商品准备情况。',
      status: laneStatus(merchantOps, productReadiness),
      actionText: '看商家运营',
      actionPath: merchantOps?.action_path || '/admin/merchants'
    },
    {
      key: 'cashier',
      label: '资金与收款',
      title: laneTitle(settlement, '看收款和结算风险'),
      summary: `${paymentRefund?.summary || '退款与支付配置待巡检。'} ${settlement?.summary || '结算和收款审核待巡检。'}`,
      status: laneStatus(paymentRefund, settlement),
      actionText: '看订阅营收',
      actionPath: '/admin/subscriptions'
    },
    {
      key: 'deploy',
      label: '上线配置',
      title: configRiskCount.value ? `还有 ${configRiskCount.value} 项配置要复核` : laneTitle(deployment, '生产配置已就绪'),
      summary: deployment?.summary || '聚合环境变量、域名、密钥、备份、限流、跨域和日志审计。',
      status: laneStatus(deployment, checks.value),
      actionText: '看配置检查',
      actionPath: '/admin/system'
    }
  ]
})
const priorityItems = computed(() => {
  const health = healthSections.value
    .filter((item) => ['danger', 'warning'].includes(item.status))
    .map((item) => ({
      source: 'health',
      group: groupLabel(item.key),
      key: item.key,
      title: item.title,
      status: item.status,
      summary: item.suggestion || item.summary,
      actionText: item.action_text || '去处理',
      actionPath: item.action_path || '/admin/system'
    }))
  const config = checks.value
    .filter((item) => ['danger', 'warning'].includes(item.status))
    .map((item) => ({
      source: 'config',
      group: '上线配置',
      key: item.key,
      title: item.title,
      status: item.status,
      summary: item.suggestion || item.description,
      actionText: '查看步骤',
      configItem: item
    }))
  return [...health, ...config]
    .sort((a, b) => statusScore(b.status) - statusScore(a.status))
    .slice(0, 8)
})
const overallType = computed(() => statusType(report.value.overall_status))
const overallLabel = computed(() => ({ pass: '可上线', warning: '需复核', danger: '暂不建议上线' })[report.value.overall_status] || '需复核')
const platformQrReady = computed(() => Boolean(form.platform_alipay_qr_code || form.platform_wechat_qr_code))
const overallDescription = computed(() => ({
  pass: '当前没有发现阻塞运行的风险，继续保持每日巡检。',
  warning: '存在需要复核的配置或运营风险，建议按板块逐项处理。',
  danger: '存在会影响收款、履约、结算或安全的风险，请优先处理。'
})[report.value.overall_status] || '请重新检查系统状态。')

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

const loadAIStatus = async () => {
  aiChecking.value = true
  try {
    const [res, usageRes] = await Promise.all([fetchAdminAIConfigStatus(), fetchAdminAIUsageOverview()])
    aiStatus.value = res.data || {}
    aiUsage.value = usageRes.data || {}
  } finally {
    aiChecking.value = false
  }
}

const save = async () => {
  await saveConfigs(form)
  ElMessage.success('配置已保存')
}

const copyText = async (text) => {
  if (!text) return
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

const uploadPlatformQr = async (options, key) => {
  const file = options.file
  if (!file) return
  if (!file.type?.startsWith('image/')) {
    ElMessage.warning('请上传图片格式的收款码')
    options.onError?.(new Error('invalid file type'))
    return
  }
  if (file.size > 3 * 1024 * 1024) {
    ElMessage.warning('收款码图片建议小于 3MB')
    options.onError?.(new Error('file too large'))
    return
  }
  uploadingQr.value = key
  try {
    const formData = new FormData()
    formData.append('file', file)
    const res = await uploadPlatformPaymentQRCode(formData)
    if (key === 'alipay') form.platform_alipay_qr_code = res.data.url || ''
    if (key === 'wechat') form.platform_wechat_qr_code = res.data.url || ''
    ElMessage.success('收款码已上传，记得保存配置')
    options.onSuccess?.(res.data)
  } catch (error) {
    options.onError?.(error)
    ElMessage.error(error?.response?.data?.message || '收款码上传失败')
  } finally {
    uploadingQr.value = ''
  }
}

const goAction = (path) => {
  router.push(path)
}

const handlePriority = (item) => {
  if (item.source === 'health') {
    goAction(item.actionPath)
    return
  }
  openGuide(item.configItem)
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

const statusType = (status) => ({ pass: 'success', warning: 'warning', danger: 'danger' }[status] || 'info')
const statusLabel = (status) => ({ pass: '通过', warning: '警告', danger: '危险' }[status] || '未知')
const statusScore = (status) => ({ danger: 3, warning: 2, pass: 1 }[status] || 0)
const groupLabel = (key) => ({
  transaction: '交易闭环',
  payment_refund: '资金与收款',
  settlement: '资金与收款',
  merchant_ops: '商家运营',
  product_readiness: '顾客下单',
  deployment: '上线配置'
}[key] || '运行风险')
const laneTitle = (section, fallback) => {
  if (!section) return fallback
  if (section.status === 'danger') return '有高优先级风险'
  if (section.status === 'warning') return '有事项需要复核'
  return fallback
}

onMounted(async () => {
  await Promise.all([loadConfigs(), loadSecurityCheck(), loadAIStatus()])
})
</script>

<style scoped>
.system-page {
  display: grid;
  gap: 20px;
}

.health-summary {
  display: grid;
  grid-template-columns: 1.3fr repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.health-summary article {
  min-height: 118px;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.07);
}

.health-summary article.pass {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.health-summary article.warning {
  border-color: #fde68a;
  background: #fffbeb;
}

.health-summary article.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.health-summary span,
.health-summary strong,
.health-summary p {
  display: block;
}

.health-summary span {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.health-summary strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 28px;
  line-height: 1.1;
}

.health-summary p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.check-grid,
.health-grid,
.ai-grid,
.script-grid,
.patrol-lanes,
.priority-list {
  display: grid;
  gap: 18px;
}

.check-grid,
.health-grid,
.ai-grid,
.patrol-lanes {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.patrol-lanes {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.patrol-lanes article {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  min-height: 138px;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.07);
}

.patrol-lanes article.warning {
  border-color: #fde68a;
  background: #fffbeb;
}

.patrol-lanes article.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.patrol-lanes article.pass {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.patrol-lanes div,
.patrol-lanes span,
.patrol-lanes strong,
.patrol-lanes p {
  display: block;
}

.patrol-lanes span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.patrol-lanes strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 18px;
}

.patrol-lanes p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.script-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.priority-list {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.priority-list article {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
  min-height: 112px;
  padding: 16px;
  border: 1px solid #fde68a;
  border-radius: 8px;
  background: #fffbeb;
}

.priority-list article.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.priority-list article div,
.priority-list article strong,
.priority-list article span {
  display: block;
}

.priority-list article strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 17px;
}

.priority-list article span {
  margin-top: 8px;
  color: #64748b;
  line-height: 1.55;
}

.check-card {
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 16px 38px rgba(15, 23, 42, 0.08);
}

.health-card {
  display: grid;
  gap: 12px;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.health-card.pass {
  border-color: #bbf7d0;
}

.health-card.warning {
  border-color: #fde68a;
  background: #fffbeb;
}

.health-card.danger {
  border-color: #fecaca;
  background: #fff1f2;
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

.check-card p {
  margin: 12px 0 8px;
  color: #334155;
  line-height: 1.7;
}

.health-card p {
  margin: 0;
  color: #334155;
  line-height: 1.65;
}

.health-card span,
.check-card span,
.guide-title span,
.dialog-desc {
  color: #64748b;
  line-height: 1.6;
}

.health-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.health-metrics div {
  min-height: 86px;
  padding: 10px;
  border: 1px solid #e5edf9;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.78);
}

.health-metrics div.warning {
  border-color: #fde68a;
  background: #fffbeb;
}

.health-metrics div.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.health-metrics small,
.health-metrics strong,
.health-metrics em {
  display: block;
}

.health-metrics small {
  color: #64748b;
  font-size: 12px;
}

.health-metrics strong {
  margin-top: 4px;
  color: #0f2747;
  font-size: 22px;
}

.health-metrics em {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  font-style: normal;
  line-height: 1.35;
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

code {
  padding: 8px 12px;
  border-radius: 10px;
  color: #dbeafe;
  background: #0f172a;
  white-space: pre-wrap;
}

.config-form {
  max-width: 820px;
}

.config-alert {
  margin-bottom: 18px;
}

.config-qr-line {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  width: 100%;
}

.platform-qr-status {
  display: flex;
  gap: 10px;
  align-items: center;
  width: 100%;
  padding: 12px;
  border: 1px solid #fde68a;
  border-radius: 8px;
  background: #fffbeb;
  color: #64748b;
}

.platform-qr-status.ready {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.ai-config-center {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 16px;
}

.ai-status-card,
.ai-env-card,
.ai-provider-grid article {
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #ffffff;
}

.ai-status-card.pass {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.ai-status-card.warning {
  border-color: #fde68a;
  background: #fffbeb;
}

.ai-status-card.danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.ai-status-card span,
.ai-status-card strong,
.ai-status-card p,
.ai-env-card span,
.ai-env-card p,
.ai-provider-grid span,
.ai-provider-grid strong,
.ai-provider-grid p,
.ai-provider-grid small {
  display: block;
}

.ai-status-card span,
.ai-env-card span,
.ai-provider-grid span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.ai-status-card strong,
.ai-provider-grid strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 22px;
}

.ai-status-card p,
.ai-env-card p,
.ai-provider-grid p,
.ai-provider-grid small {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.6;
}

.ai-status-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}

.ai-env-card pre {
  margin: 10px 0 0;
  padding: 12px;
  overflow: auto;
  color: #dbeafe;
  line-height: 1.55;
  border-radius: 8px;
  background: #0f172a;
}

.ai-provider-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.ai-usage-panel {
  margin-top: 16px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
}

.ai-usage-head {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.ai-usage-head span,
.ai-usage-head strong,
.ai-usage-head p,
.ai-usage-grid span,
.ai-usage-grid strong,
.ai-usage-grid small,
.ai-scenario-list span,
.ai-scenario-list small {
  display: block;
}

.ai-usage-head span,
.ai-scenario-list span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.ai-usage-head strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 20px;
}

.ai-usage-head p,
.ai-usage-grid small,
.ai-scenario-list small {
  margin: 6px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.ai-usage-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.ai-usage-grid article {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.ai-usage-grid span {
  color: #64748b;
  font-size: 12px;
}

.ai-usage-grid strong {
  margin-top: 6px;
  color: #0f2747;
  font-size: 22px;
}

.ai-scenario-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 14px;
}

.ai-provider-grid code {
  display: block;
  margin-top: 10px;
  font-size: 12px;
}

@media (max-width: 768px) {
  .check-grid,
  .health-grid,
  .health-summary,
  .ai-grid,
  .script-grid,
  .ai-config-center,
  .ai-usage-grid,
  .ai-provider-grid,
  .patrol-lanes,
  .priority-list,
  .config-qr-line {
    grid-template-columns: 1fr;
  }

  .patrol-lanes article,
  .priority-list article {
    flex-direction: column;
  }

  .guide-title {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (min-width: 769px) and (max-width: 1180px) {
  .check-grid,
  .health-grid,
  .health-summary,
  .ai-grid,
  .script-grid,
  .ai-config-center,
  .ai-usage-grid,
  .ai-provider-grid,
  .patrol-lanes,
  .priority-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
