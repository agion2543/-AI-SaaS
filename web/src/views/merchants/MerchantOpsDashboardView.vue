<template>
<div class="ops-dashboard">
<section class="page-card hero">
<div>
<div class="eyebrow">AI OPS DASHBOARD</div>
<h1>经营看板</h1>
<p>用真实订单、商品、顾客和活动数据出日报、做复盘、给建议。数字只按系统记录与商家自填分开标注，未接入的外部数据不写成 0。</p>
</div>
<el-button type="primary" @click="loadActiveTab">刷新</el-button>
</section>

<el-tabs v-model="activeTab" class="page-card tabs" @tab-change="loadActiveTab">
<el-tab-pane label="每日总结" name="daily">
<div class="tab-tools">
<el-date-picker v-model="dailyDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" @change="loadDaily" />
<el-button @click="loadDaily">查询</el-button>
</div>
<div v-if="dailyLoading" class="empty-hint">加载中…</div>
<div v-else-if="daily" class="daily-body">
<div class="stat-grid">
<div class="stat"><span>交易额</span><strong>{{ formatCents(daily.trade_amount_cents)}}</strong><small>来源：{{ daily.trade_source}}</small></div>
<div class="stat"><span>订单数</span><strong>{{ daily.order_count}}</strong><small>有效交易（已收款且金额大于 0）</small></div>
<div class="stat"><span>退款</span><strong>{{ formatCents(daily.refund_amount_cents)}}</strong><small>净额 {{ formatCents(daily.net_amount_cents)}}</small></div>
<div class="stat"><span>新顾客</span><strong>{{ daily.new_customers}}</strong><small>当日首次有效交易的手机号</small></div>
</div>
<el-table :data="daily.by_channel" style="width: 100%">
<el-table-column prop="channel_name" label="渠道" />
<el-table-column prop="order_count" label="订单数" width="100" />
<el-table-column label="交易额" width="160">
<template #default="{ row}">{{ formatCents(row.amount_cents)}}</template>
</el-table-column>
<el-table-column prop="source" label="来源" width="120" />
</el-table>
<div class="manual-block">
<strong>抖音团购手工核销</strong>
<span>核销 {{ daily.manual_redemption_count}} 笔 / {{ formatCents(daily.manual_redemption_amount_cents)}}</span>
<small>来源：{{ daily.manual_redemption_source}}（来客核销后人工登记）</small>
<p>{{ daily.douyin_note}}</p>
</div>
</div>
</el-tab-pane>

<el-tab-pane label="活动复盘" name="postmortem">
<div v-if="postmortemLoading" class="empty-hint">加载中…</div>
<div v-else>
<div v-if="!postmortems.length" class="empty-hint">暂无复盘。先给活动设目标，活动结束后来这里生成复盘。</div>
<article v-for="row in postmortems" :key="row.id" class="postmortem">
<header>
<strong>{{ row.campaign_type === 'promotion'? '优惠活动': '分享裂变'}} #{{ row.campaign_id}}</strong>
<span>{{ row.period_start}} ~ {{ row.period_end}}</span>
<el-tag :type="opsDeviationType(row.actual_amount_cents, row.target_amount_cents)">{{ opsDeviationText(row.actual_amount_cents, row.target_amount_cents)}}</el-tag>
</header>
<div class="compare">
<span>目标：{{ row.target_orders}} 单 / {{ formatCents(row.target_amount_cents)}}</span>
<span>实际：{{ row.actual_orders}} 单 / {{ formatCents(row.actual_amount_cents)}} / 新客 {{ row.actual_new_customers}} 人</span>
</div>
<p v-if="row.conclusion">{{ row.conclusion}}</p>
<small>实际值由系统按冻结交易口径统计后快照落库，生成后不再重算。</small>
</article>
</div>

<el-divider>生成复盘</el-divider>
<el-form :model="postmortemForm" label-width="90px" class="inline-form">
<el-form-item label="活动类型">
<el-select v-model="postmortemForm.campaign_type">
<el-option label="优惠活动" value="promotion" />
<el-option label="分享裂变" value="share_campaign" />
</el-select>
</el-form-item>
<el-form-item label="活动 ID"><el-input-number v-model="postmortemForm.campaign_id" :min="1" /></el-form-item>
<el-form-item label="开始日期"><el-date-picker v-model="postmortemForm.period_start" type="date" value-format="YYYY-MM-DD" /></el-form-item>
<el-form-item label="结束日期"><el-date-picker v-model="postmortemForm.period_end" type="date" value-format="YYYY-MM-DD" /></el-form-item>
<el-form-item label="复盘结论"><el-input v-model="postmortemForm.conclusion" type="textarea" :rows="2" placeholder="写一句结论，例如：权益力度不够，下轮换成第二件半价再试。" /></el-form-item>
<el-button type="primary" :loading="generatingPostmortem" @click="generatePostmortem">生成复盘快照</el-button>
</el-form>

<el-divider>活动目标</el-divider>
<div v-if="!targets.length" class="empty-hint">暂无目标。</div>
<el-table v-else :data="targets" style="width: 100%">
<el-table-column label="活动" width="140">
<template #default="{ row}">{{ row.campaign_type === 'promotion'? '优惠活动': '分享裂变'}} #{{ row.campaign_id}}</template>
</el-table-column>
<el-table-column label="周期" width="210">
<template #default="{ row}">{{ row.period_start}} ~ {{ row.period_end}}</template>
</el-table-column>
<el-table-column prop="target_orders" label="目标单数" width="90" />
<el-table-column label="目标交易额" width="130">
<template #default="{ row}">{{ formatCents(row.target_amount_cents)}}</template>
</el-table-column>
<el-table-column prop="target_new_customers" label="目标新客" width="90" />
</el-table>
<el-form :model="targetForm" label-width="90px" class="inline-form target-form">
<el-form-item label="活动类型">
<el-select v-model="targetForm.campaign_type">
<el-option label="优惠活动" value="promotion" />
<el-option label="分享裂变" value="share_campaign" />
</el-select>
</el-form-item>
<el-form-item label="活动 ID"><el-input-number v-model="targetForm.campaign_id" :min="1" /></el-form-item>
<el-form-item label="开始日期"><el-date-picker v-model="targetForm.period_start" type="date" value-format="YYYY-MM-DD" /></el-form-item>
<el-form-item label="结束日期"><el-date-picker v-model="targetForm.period_end" type="date" value-format="YYYY-MM-DD" /></el-form-item>
<el-form-item label="目标单数"><el-input-number v-model="targetForm.target_orders" :min="0" /></el-form-item>
<el-form-item label="目标交易额（元）"><el-input-number v-model="targetForm.target_amount_yuan" :min="0" :precision="2" /></el-form-item>
<el-form-item label="目标新客"><el-input-number v-model="targetForm.target_new_customers" :min="0" /></el-form-item>
<el-button type="primary" :loading="savingTarget" @click="saveTarget">保存目标</el-button>
</el-form>
</el-tab-pane>

<el-tab-pane label="商品体检" name="product">
<div class="tab-tools">
<el-select v-model="healthDays" @change="loadProductHealth">
<el-option :value="7" label="近 7 天" />
<el-option :value="30" label="近 30 天" />
<el-option :value="90" label="近 90 天" />
</el-select>
<el-button @click="loadProductHealth">刷新</el-button>
<small class="hint">按 order_items 明细汇总，来源：{{ health?.source || '系统记录'}}</small>
</div>
<div v-if="healthLoading" class="empty-hint">加载中…</div>
<div v-else-if="!health?.rows?.length" class="empty-hint">窗口期内暂无商品销售数据。</div>
<el-table v-else :data="health.rows" style="width: 100%">
<el-table-column prop="product_name" label="商品" />
<el-table-column prop="quantity" label="销量" width="80" />
<el-table-column label="交易额" width="130">
<template #default="{ row}">{{ formatCents(row.amount_cents)}}</template>
</el-table-column>
<el-table-column prop="last_sold_at" label="最后售出" width="120" />
<el-table-column label="体检" width="100">
<template #default="{ row}"><el-tag :type="opsProductStatusType(row.status)">{{ row.status_name}}</el-tag></template>
</el-table-column>
<el-table-column prop="suggestion" label="建议" min-width="260" />
</el-table>
</el-tab-pane>

<el-tab-pane label="沉睡顾客" name="dormant">
<div v-if="dormantLoading" class="empty-hint">加载中…</div>
<div v-else-if="dormant" class="dormant-body">
<p class="hint">按最后一次有效交易的距今天数分层；手机号仅展示打码抽样。唤醒草稿是模板文案，供商家自己发给顾客，内权益由商家填写。</p>
<div v-for="tier in dormant.tiers" :key="tier.tier" class="tier-card">
<header>
<strong>{{ tier.tier_name}}</strong>
<span>{{ tier.count}} 人</span>
</header>
<small v-if="tier.sample_phones?.length">抽样：{{ tier.sample_phones.join('、')}}</small>
<small v-else>本层暂无顾客。</small>
<p>{{ tier.draft}}</p>
<div class="tier-actions">
<el-tag type="info">{{ tier.draft_source}}</el-tag>
<el-button size="small" @click="copyText(tier.draft)">复制草稿</el-button>
</div>
</div>
</div>
</el-tab-pane>

<el-tab-pane label="内容脚本" name="content">
<div class="tab-tools">
<el-select v-model="scriptScenario">
<el-option label="新品上市" value="new_dish" />
<el-option label="促销活动" value="promotion" />
<el-option label="门店故事" value="store_story" />
<el-option label="节日问候" value="festival" />
</el-select>
<el-button type="primary" :loading="scriptLoading" @click="loadContentScript">生成脚本</el-button>
<small class="hint">只写商家自己账号的宣传脚本，不写顾客评价、不编销量价格。</small>
</div>
<div v-if="!script" class="empty-hint">选一个场景，点“生成脚本”。</div>
<div v-else class="script-body">
<header>
<strong>{{ script.title}}</strong>
<el-tag v-if="script.fallback" type="warning">模板兜底</el-tag>
<el-tag v-else type="success">模型生成</el-tag>
<small>来源：{{ script.source}}</small>
</header>
<p class="hook">{{ script.hook}}</p>
<ol>
<li v-for="(line, i) in script.lines" :key="i">{{ line}}</li>
</ol>
<p class="caption">发布文案：{{ script.caption}}</p>
<el-button size="small" @click="copyText(script.lines.join('\n') + '\n' + script.caption)">复制脚本</el-button>
</div>
</el-tab-pane>

<el-tab-pane label="建议执行" name="suggestions">
<div class="tab-tools">
<el-button type="primary" :loading="generatingSuggestions" @click="generateSuggestions">从数据生成建议</el-button>
<small class="hint">按商品体检、沉睡分层与复盘偏差生成待办；重复点击不会重复生成。</small>
</div>
<div v-if="suggestionsLoading" class="empty-hint">加载中…</div>
<div v-else-if="!suggestions.length" class="empty-hint">暂无建议。点“从数据生成建议”，按数据生成待办。</div>
<article v-for="row in suggestions" :key="row.id" class="suggestion">
<header>
<strong>{{ row.title}}</strong>
<el-tag :type="row.status === 'executed'? 'success': row.status === 'ignored'? 'info': row.status === 'adopted'? '': 'warning'">{{ opsSuggestionStatusLabel(row.status)}}</el-tag>
</header>
<p>{{ row.content_snapshot}}</p>
<small>来源：{{ row.source}} · 生成时间 {{ row.created_at}}</small>
<div v-if="row.status === 'adopted'" class="effect-row">
<el-input v-model="effectNotes[row.id]" placeholder="执行后填一句效果，例如：已上架第二件半价，三天多卖 12 单（效果由商家填写）" />
</div>
<div class="suggestion-actions">
<el-button
v-for="action in opsNextSuggestionActions(row.status)"
 :key="action.status"
size="small"
 :type="action.status === 'adopted'? 'primary': ''"
@click="updateSuggestion(row, action.status)"
>{{ action.label}}</el-button>
<small v-if="row.effect_note">效果：{{ row.effect_note}}</small>
</div>
</article>
</el-tab-pane>
</el-tabs>
</div>
</template>

<script setup>
import { onMounted, reactive, ref} from 'vue'
import { ElMessage} from 'element-plus'
import {
fetchOpsDailySummary,
fetchOpsProductHealth,
fetchOpsDormantCustomers,
fetchOpsContentScript,
generateOpsSuggestions,
fetchMerchantCampaignPostmortems,
fetchMerchantCampaignTargets,
createMerchantCampaignTarget,
createMerchantCampaignPostmortem,
fetchMerchantSuggestionExecutions,
updateMerchantSuggestionExecution,
} from '../../api/modules'
import { formatCents} from '../../utils/aiCostControl'
import {
opsDeviationText,
opsDeviationType,
opsProductStatusType,
opsSuggestionStatusLabel,
opsNextSuggestionActions,
opsTodayDate,
} from '../../utils/opsDashboard'

const activeTab = ref('daily')
const dailyDate = ref(opsTodayDate())
const daily = ref(null)
const dailyLoading = ref(false)
const postmortems = ref([])
const targets = ref([])
const postmortemLoading = ref(false)
const generatingPostmortem = ref(false)
const savingTarget = ref(false)
const health = ref(null)
const healthDays = ref(30)
const healthLoading = ref(false)
const dormant = ref(null)
const dormantLoading = ref(false)
const scriptScenario = ref('new_dish')
const script = ref(null)
const scriptLoading = ref(false)
const suggestions = ref([])
const suggestionsLoading = ref(false)
const generatingSuggestions = ref(false)
const effectNotes = reactive({})

const postmortemForm = reactive({
campaign_type: 'promotion',
campaign_id: null,
period_start: '',
period_end: '',
conclusion: '',
})
const targetForm = reactive({
campaign_type: 'promotion',
campaign_id: null,
period_start: '',
period_end: '',
target_orders: 0,
target_amount_yuan: 0,
target_new_customers: 0,
})

const fail = (err, fallback) => ElMessage.error(err?.response?.data?.message || fallback)

const loadDaily = async () => {
dailyLoading.value = true
try {
const res = await fetchOpsDailySummary({ date: dailyDate.value})
daily.value = res?.data || null
} catch (err) {
fail(err, '加载每日总结失败')
} finally {
dailyLoading.value = false
}
}

const loadPostmortems = async () => {
postmortemLoading.value = true
try {
const [pmRes, targetRes] = await Promise.all([
fetchMerchantCampaignPostmortems({ page_size: 20}),
fetchMerchantCampaignTargets({ page_size: 20}),
])
postmortems.value = pmRes?.data?.rows || []
targets.value = targetRes?.data?.rows || []
} catch (err) {
fail(err, '加载活动复盘失败')
} finally {
postmortemLoading.value = false
}
}

const generatePostmortem = async () => {
if (!postmortemForm.campaign_id ||!postmortemForm.period_start ||!postmortemForm.period_end) {
ElMessage.warning('请先填活动 ID 与复盘周期')
return
}
generatingPostmortem.value = true
try {
await createMerchantCampaignPostmortem({...postmortemForm})
ElMessage.success('复盘快照已生成')
await loadPostmortems()
} catch (err) {
fail(err, '生成复盘失败')
} finally {
generatingPostmortem.value = false
}
}

const saveTarget = async () => {
if (!targetForm.campaign_id ||!targetForm.period_start ||!targetForm.period_end) {
ElMessage.warning('请先填活动 ID 与目标周期')
return
}
savingTarget.value = true
try {
await createMerchantCampaignTarget({
campaign_type: targetForm.campaign_type,
campaign_id: targetForm.campaign_id,
period_start: targetForm.period_start,
period_end: targetForm.period_end,
target_orders: targetForm.target_orders,
target_amount_cents: Math.round((Number(targetForm.target_amount_yuan) || 0) * 100),
target_new_customers: targetForm.target_new_customers,
})
ElMessage.success('目标已保存')
await loadPostmortems()
} catch (err) {
fail(err, '保存目标失败')
} finally {
savingTarget.value = false
}
}

const loadProductHealth = async () => {
healthLoading.value = true
try {
const res = await fetchOpsProductHealth({ days: healthDays.value})
health.value = res?.data || null
} catch (err) {
fail(err, '加载商品体检失败')
} finally {
healthLoading.value = false
}
}

const loadDormant = async () => {
dormantLoading.value = true
try {
const res = await fetchOpsDormantCustomers()
dormant.value = res?.data || null
} catch (err) {
fail(err, '加载沉睡顾客失败')
} finally {
dormantLoading.value = false
}
}

const loadContentScript = async () => {
scriptLoading.value = true
try {
const res = await fetchOpsContentScript({ scenario: scriptScenario.value})
script.value = res?.data || null
} catch (err) {
fail(err, '生成脚本失败')
} finally {
scriptLoading.value = false
}
}

const loadSuggestions = async () => {
suggestionsLoading.value = true
try {
const res = await fetchMerchantSuggestionExecutions({ page_size: 20})
suggestions.value = res?.data?.rows || []
} catch (err) {
fail(err, '加载建议失败')
} finally {
suggestionsLoading.value = false
}
}

const generateSuggestions = async () => {
generatingSuggestions.value = true
try {
const res = await generateOpsSuggestions()
const created = res?.data?.created?? 0
const skipped = res?.data?.skipped?? 0
ElMessage.success(`已生成 ${created} 条建议，跳过 ${skipped} 条重复建议`)
await loadSuggestions()
} catch (err) {
fail(err, '生成建议失败')
} finally {
generatingSuggestions.value = false
}
}

const updateSuggestion = async (row, status) => {
try {
const payload = { status}
if (status === 'executed' && effectNotes[row.id]) {
payload.effect_note = effectNotes[row.id]
}
await updateMerchantSuggestionExecution(row.id, payload)
ElMessage.success('已更新')
await loadSuggestions()
} catch (err) {
fail(err, '更新建议失败')
}
}

const copyText = async (text) => {
try {
await navigator.clipboard.writeText(text)
ElMessage.success('已复制')
} catch (err) {
const area = document.createElement('textarea')
area.value = text
document.body.appendChild(area)
area.select()
document.execCommand('copy')
document.body.removeChild(area)
ElMessage.success('已复制')
}
}

const loadActiveTab = () => {
if (activeTab.value === 'daily') loadDaily()
else if (activeTab.value === 'postmortem') loadPostmortems()
else if (activeTab.value === 'product') loadProductHealth()
else if (activeTab.value === 'dormant') loadDormant()
else if (activeTab.value === 'suggestions') loadSuggestions()
}

onMounted(loadDaily)
</script>

<style scoped>
.ops-dashboard{display:grid;gap:18px}
.page-card{padding:20px;border:1px solid #dce8f5;border-radius:18px;background:#fff}
.hero{display:flex;justify-content:space-between;align-items:center;gap:16px}
.eyebrow{color:#2563eb;font-size:12px;font-weight:900;letter-spacing:.14em}
.hero h1{margin:6px 0;font-size:24px}
.hero p{margin:0;color:#64748b;font-size:13px;line-height:1.6}
.tabs{padding-top:6px}
.tab-tools{display:flex;gap:10px;align-items:center;margin:6px 0 14px;flex-wrap:wrap}
.hint{color:#94a3b8;font-size:12px}
.empty-hint{color:#94a3b8;font-size:13px;text-align:center;padding:12px 0}
.stat-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-bottom:14px}
.stat{display:grid;gap:4px;padding:14px;border:1px solid #e2e8f0;border-radius:12px}
.stat span{color:#64748b;font-size:12px}
.stat strong{font-size:20px}
.stat small{color:#94a3b8;font-size:11px}
.manual-block{display:grid;gap:6px;margin-top:14px;padding:14px;border:1px solid #fef3c7;border-radius:12px;background:#fffbeb;font-size:13px}
.manual-block p{margin:0;color:#64748b;font-size:12px;line-height:1.6}
.postmortem{padding:14px 0;border-top:1px solid #e2e8f0}
.postmortem:first-child{border-top:0}
.postmortem header{display:flex;gap:10px;align-items:center;flex-wrap:wrap}
.postmortem header strong{flex:1}
.postmortem.compare{display:flex;gap:18px;flex-wrap:wrap;margin:8px 0;color:#334155;font-size:13px}
.postmortem p{margin:6px 0;color:#334155;font-size:13px;line-height:1.7}
.postmortem small{color:#94a3b8;font-size:12px}
.inline-form{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 18px;margin-top:8px}
.target-form{margin-top:14px}
.tier-card{display:grid;gap:8px;padding:16px;margin-top:12px;border:1px solid #e2e8f0;border-radius:12px}
.tier-card header{display:flex;justify-content:space-between;align-items:center}
.tier-card p{margin:0;color:#334155;font-size:13px;line-height:1.7}
.tier-card small{color:#64748b;font-size:12px}
.tier-actions{display:flex;gap:10px;align-items:center}
.script-body{display:grid;gap:10px}
.script-body header{display:flex;gap:10px;align-items:center}
.script-body.hook{font-weight:700;color:#1e293b}
.script-body ol{margin:0;padding-left:20px;color:#334155;font-size:13px;line-height:1.9}
.script-body.caption{color:#64748b;font-size:13px}
.suggestion{padding:14px 0;border-top:1px solid #e2e8f0}
.suggestion:first-child{border-top:0}
.suggestion header{display:flex;gap:10px;align-items:center}
.suggestion header strong{flex:1}
.suggestion p{margin:8px 0;color:#334155;font-size:13px;line-height:1.7;white-space:pre-wrap}
.suggestion small{color:#94a3b8;font-size:12px}
.suggestion-actions{display:flex;gap:10px;align-items:center;margin-top:8px}
.effect-row{margin-top:8px}
.dormant-body.hint{margin:0 0 4px}
</style>
