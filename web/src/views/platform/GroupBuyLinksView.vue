<template>
  <div class="page-wrap">
    <header class="page-heading">
      <div><p class="eyebrow">PLATFORM OPERATIONS · REVIEW RULES V1</p><h1>团购入口审核</h1><p class="intro">精确白名单自动放行；不确定项进入人工队列；危险或跨平台链接拒绝。</p></div>
      <div class="heading-actions"><el-button :type="exceptionOnly ? 'warning' : 'default'" @click="toggleExceptionQueue">{{ exceptionOnly ? '退出异常队列' : '人工异常队列' }}<el-badge v-if="summary.manual_review_count" :value="summary.manual_review_count" class="queue-badge" /></el-button><el-button @click="rulesDrawer = true">规则说明</el-button><el-button :loading="loading" @click="loadList">刷新</el-button></div>
    </header>
    <el-alert type="info" :closable="false" show-icon class="notice">本审核仅管理 SaaS 内跳转入口，不代表平台官方资质认证或 API 审核；不会访问外部链接。点击量不代表订单、核销或收入。</el-alert>
    <section class="decision-strip">
      <div class="decision-card pass"><i></i><div><strong>自动通过</strong><p>HTTPS 精确白名单 · 商家和门店均启用</p></div></div>
      <div class="decision-card manual"><i></i><div><strong>人工复核</strong><p>短链/未登记子域/已通过改链/状态异常</p></div></div>
      <div class="decision-card reject"><i></i><div><strong>自动拒绝</strong><p>仿冒域名/跨平台/未知域名/危险 URL</p></div></div>
      <el-button class="policy-link" link type="primary" @click="rulesDrawer = true">查看规则与异常矩阵 →</el-button>
    </section>
    <div class="summary-grid">
      <div class="summary-card"><span>入口总数</span><strong>{{ summary.total_links ?? 0 }}</strong></div>
      <div class="summary-card warning"><span>人工待处理</span><strong>{{ summary.manual_review_count ?? 0 }}</strong></div>
      <div class="summary-card success"><span>自动通过</span><strong>{{ summary.auto_approved_count ?? 0 }}</strong></div>
      <div class="summary-card danger"><span>自动拒绝</span><strong>{{ summary.auto_rejected_count ?? 0 }}</strong></div>
      <div class="summary-card danger"><span>平台暂停</span><strong>{{ summary.suspended_count ?? 0 }}</strong></div>
      <div class="summary-card"><span>累计入口点击</span><strong>{{ Number(summary.click_total || 0).toLocaleString('zh-CN') }}</strong></div>
    </div>
    <section class="panel filters">
      <el-input v-model="filters.keyword" placeholder="搜索商家、门店、域名或入口码" clearable @keyup.enter="search" @clear="search" />
      <el-select v-model="filters.channel" placeholder="全部平台" clearable><el-option label="美团 / 大众点评" value="meituan" /><el-option label="抖音" value="douyin" /></el-select>
      <el-select v-model="filters.review_status" placeholder="全部审核状态" clearable><el-option label="待人工复核" value="pending" /><el-option label="已通过" value="approved" /><el-option label="已拒绝" value="rejected" /></el-select>
      <el-select v-model="filters.review_method" placeholder="全部审核来源" clearable><el-option label="自动审核" value="auto" /><el-option label="人工审核" value="manual" /></el-select>
      <el-select v-model="filters.entry_state" placeholder="全部入口状态" clearable><el-option label="当前可跳转" value="active" /><el-option label="当前不可跳转" value="paused" /><el-option label="平台暂停" value="suspended" /></el-select>
      <el-button type="primary" @click="search">查询</el-button><el-button @click="resetFilters">重置</el-button>
    </section>
    <section class="panel table-panel">
      <div class="table-heading"><div><strong>{{ exceptionOnly ? '人工异常待处理' : '团购入口记录' }}</strong><span>{{ exceptionOnly ? '处理前请核对异常原因、域名和商家/门店状态。' : '单击记录查看命中规则、处理理由与追加式审核历史。' }}</span></div><el-tag v-if="exceptionOnly" type="warning">人工队列 · {{ total }}</el-tag></div>
      <el-table v-loading="loading" :data="rows" row-key="id" @row-click="openDetail">
        <el-table-column label="商家 / 门店" min-width="180"><template #default="{row}"><strong>{{ row.merchant_name || `商家 #${row.merchant_id}` }}</strong><small>{{ row.store_name || `门店 #${row.store_id}` }}</small></template></el-table-column>
        <el-table-column label="平台 / 域名" min-width="145"><template #default="{row}">{{ channelName(row.channel) }}<small>{{ row.domain || '—' }}</small></template></el-table-column>
        <el-table-column label="审核结果 / 规则" min-width="150"><template #default="{row}"><el-tag :type="reviewType(row.review_status)" effect="light">{{ reviewName(row.review_status) }}</el-tag><small>{{ methodName(row.review_method) }} · {{ row.review_rule_id || '旧记录' }}</small></template></el-table-column>
        <el-table-column label="入口状态 / 原因" min-width="155"><template #default="{row}"><el-tag :type="entryType(row)" effect="light">{{ entryName(row) }}</el-tag><small class="reason-preview">{{ row.review_reason || '—' }}</small></template></el-table-column>
        <el-table-column label="点击量" width="85" align="right"><template #default="{row}">{{ Number(row.click_count || 0).toLocaleString('zh-CN') }}</template></el-table-column>
        <el-table-column label="最近变更" min-width="160"><template #default="{row}">{{ formatTime(row.last_changed_at || row.reviewed_at || row.created_at) }}<small>{{ actionName(row.last_action) }} · {{ row.last_changed_by || row.reviewed_by || '—' }}</small></template></el-table-column>
        <el-table-column label="操作" width="70" fixed="right"><template #default="{row}"><el-button link type="primary" @click.stop="openDetail(row)">详情</el-button></template></el-table-column>
        <template #empty><el-empty :description="exceptionOnly ? '当前没有待人工核验入口' : '暂无团购入口'" /></template>
      </el-table>
      <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50]" layout="total, sizes, prev, pager, next" @current-change="loadList" @size-change="changeSize" />
    </section>
    <el-drawer v-model="drawer" title="团购入口详情" size="min(600px, 100%)" destroy-on-close>
      <div v-if="detail" class="detail-content">
        <div class="detail-title"><div><h2>{{ detail.merchant_name || `商家 #${detail.merchant_id}` }}</h2><p>{{ detail.store_name || `门店 #${detail.store_id}` }} · {{ channelName(detail.channel) }}</p></div><el-tag :type="reviewType(detail.review_status)">{{ reviewName(detail.review_status) }}</el-tag></div>
        <div class="verdict-box" :class="verdictClass(detail)"><div class="verdict-top"><strong>{{ methodName(detail.review_method) }}</strong><el-tag effect="plain" size="small">{{ detail.review_rule_id || '旧记录 / 未记录规则' }}</el-tag></div><p>{{ detail.review_reason || '该记录创建于规则留痕上线前，暂无自动判定原因。' }}</p></div>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="商家 / 门店状态">{{ detail.merchant_status || '—' }} / {{ detail.store_status || '—' }}</el-descriptions-item>
          <el-descriptions-item label="入口状态"><el-tag :type="entryType(detail)">{{ entryName(detail) }}</el-tag></el-descriptions-item>
          <el-descriptions-item label="目标链接"><span class="url-text">{{ detail.destination_url || '—' }}</span></el-descriptions-item>
          <el-descriptions-item label="域名 / 入口码">{{ detail.domain || '—' }}<template v-if="detail.code"> · {{ detail.code }}</template></el-descriptions-item>
          <el-descriptions-item label="点击量 / 最近点击">{{ Number(detail.click_count || 0).toLocaleString('zh-CN') }} 次 · {{ formatTime(detail.last_clicked_at) }}</el-descriptions-item>
          <el-descriptions-item label="审核人 / 时间">{{ detail.reviewed_by || (detail.review_method === 'auto' ? '系统规则' : '—') }} · {{ formatTime(detail.reviewed_at) }}</el-descriptions-item>
          <el-descriptions-item v-if="detail.suspension_reason" label="平台暂停原因">{{ detail.suspension_reason }}</el-descriptions-item>
        </el-descriptions>
        <section class="history"><h3>操作与规则审计</h3><el-timeline v-if="history.length"><el-timeline-item v-for="event in history" :key="event.id" :timestamp="formatTime(event.created_at)" placement="top"><div class="history-title"><strong>{{ actionName(event.action) }}</strong><el-tag v-if="event.rule_id" size="small" effect="plain">{{ event.rule_id }}</el-tag></div><p>{{ event.actor_name || event.actor_type || '—' }}<template v-if="event.reason"> · {{ event.reason }}</template></p><small>变更：{{ stateDiff(event) }}</small></el-timeline-item></el-timeline><el-empty v-else description="暂无历史记录" :image-size="70" /></section>
        <div class="action-bar"><el-button v-if="detail.review_status === 'pending'" type="success" :loading="acting" @click="approve">人工通过</el-button><el-button v-if="detail.review_status === 'pending'" type="danger" plain :loading="acting" @click="reject">拒绝</el-button><el-button v-if="detail.review_status === 'approved' && !detail.admin_suspended" type="warning" plain :loading="acting" @click="perform('suspend')">平台暂停</el-button><el-button v-if="detail.review_status === 'approved' && detail.admin_suspended" type="primary" :loading="acting" @click="perform('resume')">恢复入口</el-button></div>
        <p class="footnote">人工通过须填写理由，且会再次校验 HTTPS、平台域名家族及商家/门店状态。平台暂停、商家停用或门店未启用时仍不可跳转。本审核不是官方资质或 API 审核。</p>
      </div>
    </el-drawer>
    <el-drawer v-model="rulesDrawer" title="团购自动审核规则与异常矩阵" size="min(760px, 100%)" destroy-on-close>
      <div class="rules-content">
        <el-alert type="warning" :closable="false" show-icon title="这是站内跳转入口风控，不是抖音/美团官方审核；不请求外部 URL，不接入平台 API。" />
        <div class="allowlist"><strong>精确自动通过域名</strong><span>抖音：life.douyin.com</span><span>美团：i.meituan.com</span><span>大众点评：www.dianping.com</span><small>v.douyin.com 短链、未登记子域和根页均转人工；规则不会自动扩充。</small></div>
        <h3>判定规则</h3><el-table :data="ruleRows" size="small" border><el-table-column prop="id" label="规则 ID" width="155" /><el-table-column prop="condition" label="条件" min-width="230" /><el-table-column prop="result" label="处理" width="100" /><el-table-column prop="effect" label="后续" min-width="150" /></el-table>
        <h3>异常状态矩阵</h3><el-table :data="stateRows" size="small" border><el-table-column prop="state" label="状态" width="160" /><el-table-column prop="redirect" label="可跳转" width="95" /><el-table-column prop="action" label="处理" min-width="260" /></el-table>
        <h3>合成测试覆盖</h3><div class="test-tags"><el-tag v-for="item in testCases" :key="item" effect="plain" type="info">{{ item }}</el-tag></div>
        <p class="rules-note">自动判定和管理员决定均追加记录规则编号（适用时）、结果/原因、操作者、时间及前后状态。安全预检失败的请求在落库前阻断。完整表格见项目文档 docs/group-buy-auto-review.md。</p>
      </div>
    </el-drawer>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fetchAdminGroupBuyLinks, fetchAdminGroupBuyLinkDetail, reviewAdminGroupBuyLink } from '../../api/modules'
const rows = ref([]), total = ref(0), loading = ref(false), acting = ref(false), drawer = ref(false), rulesDrawer = ref(false), detail = ref(null), history = ref([]), exceptionOnly = ref(false), page = ref(1), pageSize = ref(20)
const summary = reactive({ total_links: 0, pending_count: 0, manual_review_count: 0, auto_approved_count: 0, auto_rejected_count: 0, suspended_count: 0, click_total: 0 })
const filters = reactive({ keyword: '', channel: '', review_status: '', review_method: '', entry_state: '' })
const ruleRows = [
  { id: 'GBR-SEC-001', condition: 'HTTP、凭据、显式端口、IP、畸形/过长 URL', result: '拒绝提交', effect: '预检阻断，不保存' },
  { id: 'GBR-CHAN-002', condition: '渠道与另一平台域名不符', result: '自动拒绝', effect: '留审计，不可跳转' },
  { id: 'GBR-DOMAIN-001/002', condition: '疑似仿冒或未知域名', result: '自动拒绝', effect: '管理员不可绕过' },
  { id: 'GBR-DOMAIN-003 / LINK-001', condition: '未登记子域、短链或平台根页', result: '人工复核', effect: '进入异常队列' },
  { id: 'GBR-CHANGE-001', condition: '已通过链接目标发生变更', result: '人工复核', effect: '旧批准立即失效' },
  { id: 'GBR-STATE-001', condition: '商家/门店缺失、未启用', result: '人工复核', effect: '状态门禁持续生效' },
  { id: 'GBR-AUTO-001', condition: '精确白名单 + HTTPS + 状态正常', result: '自动通过', effect: '记录系统规则审计' },
  { id: 'GBR-ADMIN-001/002', condition: '人工通过/拒绝/暂停/恢复', result: '人工决定', effect: '理由必填并追加审计' }
]
const stateRows = [
  { state: 'approved · auto', redirect: '条件满足时是', action: '入口启用、商家/门店 active、未暂停；改链转人工' },
  { state: 'pending · manual', redirect: '否', action: '核对异常原因后，填写理由通过或拒绝' },
  { state: 'rejected · auto/manual', redirect: '否', action: '改链重提；危险或域名错误不能手工绕过' },
  { state: 'approved + paused/disabled', redirect: '否', action: '暂停/停用门禁仍生效，恢复前复核状态' }
]
const testCases = ['T01–03 精确白名单通过', 'T04 未登记子域复核', 'T05 短链复核', 'T06 根页复核', 'T07 跨平台拒绝', 'T08 仿冒域拒绝', 'T09 未知域拒绝', 'T10 危险 URL 阻断', 'T11 状态异常复核', 'T12 通过后改链', 'T13 人工理由与安全门禁', 'T14 租户隔离', 'T15 审计轨迹']
const loadList = async () => { loading.value = true; try { const { data } = await fetchAdminGroupBuyLinks({ page: page.value, page_size: pageSize.value, ...filters }); rows.value = data.list || []; total.value = Number(data.total || 0); Object.assign(summary, data.summary || {}) } catch (e) { ElMessage.error(e?.response?.data?.message || '加载入口列表失败') } finally { loading.value = false } }
const search = () => { page.value = 1; loadList() }
const resetFilters = () => { Object.assign(filters, { keyword: '', channel: '', review_status: '', review_method: '', entry_state: '' }); exceptionOnly.value = false; search() }
const changeSize = () => { page.value = 1; loadList() }
const toggleExceptionQueue = () => { exceptionOnly.value = !exceptionOnly.value; if (exceptionOnly.value) Object.assign(filters, { review_status: 'pending', review_method: 'manual' }); else Object.assign(filters, { review_status: '', review_method: '' }); search() }
const openDetail = async row => { drawer.value = true; detail.value = row; history.value = []; try { const { data } = await fetchAdminGroupBuyLinkDetail(row.id); detail.value = data.link || row; history.value = data.history || [] } catch (e) { ElMessage.error(e?.response?.data?.message || '加载详情失败') } }
const act = async (action, reason) => { if (!detail.value || acting.value) return; acting.value = true; try { await reviewAdminGroupBuyLink(detail.value.id, { action, ...(reason ? { reason } : {}) }); ElMessage.success('操作已完成，审计记录已追加'); const current = { ...detail.value }; await loadList(); await openDetail(current) } catch (e) { ElMessage.error(e?.response?.data?.message || '操作失败') } finally { acting.value = false } }
const approve = async () => { try { const { value } = await ElMessageBox.prompt('请填写核验依据；系统会再次检查 URL 安全与商家/门店状态。', '人工审核通过', { inputType: 'textarea', inputPlaceholder: '例如：已核对平台后台团购落地页与门店信息', inputValidator: v => Boolean(v?.trim()) || '通过审核必须填写核验理由', confirmButtonText: '确认通过', cancelButtonText: '取消' }); await act('approve', value.trim()) } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '操作失败') } }
const reject = async () => { try { const { value } = await ElMessageBox.prompt('请说明拒绝原因（必填）', '拒绝团购入口', { inputType: 'textarea', inputPlaceholder: '填写拒绝理由', inputValidator: v => Boolean(v?.trim()) || '拒绝时必须填写理由', confirmButtonText: '确认拒绝', cancelButtonText: '取消' }); await act('reject', value.trim()) } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '操作失败') } }
const perform = async action => {
  let reason = ''
  if (action === 'suspend' || action === 'resume') {
    const title = action === 'suspend' ? '平台暂停' : '恢复入口'
    const prompt = action === 'suspend' ? '请填写平台暂停原因。' : '请填写恢复原因；商家和门店状态门禁仍生效。'
    try {
      const result = await ElMessageBox.prompt(prompt, title, { inputType: 'textarea', inputPlaceholder: '填写操作理由', inputValidator: value => Boolean(value?.trim()) || '必须填写操作理由', confirmButtonText: action === 'suspend' ? '确认暂停' : '确认恢复', cancelButtonText: '取消' })
      reason = result.value.trim()
    } catch { return }
  }
  await act(action, reason)
}
const channelName = v => ({ meituan: '美团 / 大众点评', douyin: '抖音' }[v] || v || '—')
const reviewName = v => ({ pending: '待人工复核', approved: '已通过', rejected: '已拒绝' }[v] || v || '—')
const methodName = v => ({ auto: '系统自动', manual: '人工处理' }[v] || '旧记录')
const reviewType = v => ({ pending: 'warning', approved: 'success', rejected: 'danger' }[v] || 'info')
const verdictClass = r => r.review_status === 'approved' ? 'verdict-success' : r.review_status === 'rejected' ? 'verdict-danger' : 'verdict-warning'
const entryName = r => { if (r.admin_suspended) return '平台暂停'; if (!r.enabled) return '商家已停用'; if (r.review_status === 'pending') return '待复核，不可跳转'; if (r.review_status === 'rejected') return '审核未通过'; if (r.merchant_status && r.merchant_status !== 'active') return '商家未启用'; if (r.store_status && r.store_status !== 'active') return '门店未启用'; return '当前可跳转' }
const entryType = r => entryName(r) === '当前可跳转' ? 'success' : r.admin_suspended || r.review_status === 'rejected' ? 'danger' : 'warning'
const formatTime = v => v ? new Date(v).toLocaleString('zh-CN', { hour12: false }) : '—'
const actionName = v => ({ approve: '人工审核通过', platform_approve: '人工审核通过', reject: '人工审核拒绝', platform_reject: '人工审核拒绝', suspend: '平台暂停', platform_suspend: '平台暂停', resume: '恢复入口', platform_resume: '恢复入口', system_approved: '系统自动通过', system_pending: '转人工复核', system_rejected: '系统自动拒绝', merchant_link_created: '商家提交入口', merchant_destination_changed: '商家改链并重新审核', merchant_link_updated: '商家调整入口状态', demo_seeded: '演示数据初始化' }[v] || v || '状态变更')
const parseState = raw => { if (!raw) return {}; if (typeof raw === 'object') return raw; try { return JSON.parse(raw) } catch { return {} } }
const stateValue = (key, value) => { if (value === undefined || value === null) return '未设置'; if (key === 'destination_url') { try { return value ? new URL(value).hostname : '空' } catch { return value || '空' } } if (key === 'review_status') return reviewName(value); if (key === 'review_method') return methodName(value); if (key === 'enabled') return value ? '商家启用' : '商家停用'; if (key === 'admin_suspended') return value ? '平台暂停' : '平台未暂停'; return value == null || value === '' ? '无' : String(value) }
const stateDiff = event => { const before = parseState(event.before_state), after = parseState(event.after_state); const labels = { destination_url: '目标域名', review_status: '审核状态', review_method: '审核方式', review_rule_id: '规则编号', review_reason: '审核原因', enabled: '商家启用状态', admin_suspended: '平台暂停状态', suspension_reason: '暂停原因' }; const keys = [...new Set([...Object.keys(before), ...Object.keys(after)])]; const changes = keys.filter(k => JSON.stringify(before[k]) !== JSON.stringify(after[k])); return changes.map(k => `${labels[k] || k}：${stateValue(k, before[k])} → ${stateValue(k, after[k])}`).join('；') || '记录了状态变化' }
onMounted(loadList)
</script>
<style scoped>
.page-wrap{display:grid;gap:15px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:14px}.eyebrow{margin:0 0 5px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b}.heading-actions{display:flex;gap:8px;flex-wrap:wrap}.queue-badge{margin-left:7px}.notice{border-radius:12px}.decision-strip{position:relative;display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px;padding:12px 170px 12px 12px;background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.decision-card{display:flex;gap:9px;padding:8px 10px;border-radius:10px;background:#f8fafc}.decision-card i{width:9px;height:9px;margin-top:4px;border-radius:50%;background:#94a3b8}.decision-card.pass i{background:#16a34a}.decision-card.manual i{background:#d97706}.decision-card.reject i{background:#dc2626}.decision-card strong{font-size:13px}.decision-card p{margin:4px 0 0;color:#64748b;font-size:11px;line-height:1.45}.policy-link{position:absolute;right:14px;top:50%;transform:translateY(-50%)}.summary-grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:9px}.summary-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.summary-card{padding:12px 14px;display:grid;gap:7px}.summary-card span{font-size:12px;color:#64748b}.summary-card strong{font-size:22px;color:#17365d}.summary-card.warning strong{color:#d97706}.summary-card.success strong{color:#15803d}.summary-card.danger strong{color:#dc2626}.panel{padding:13px}.filters{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.filters .el-input{width:240px}.filters .el-select{width:140px}.table-panel{padding:8px 12px 14px}.table-heading{display:flex;justify-content:space-between;align-items:center;gap:10px;padding:9px 6px 12px}.table-heading div{display:grid;gap:4px}.table-heading strong{font-size:15px}.table-heading span{color:#7b8798;font-size:12px}.table-panel :deep(.el-table){cursor:pointer}.table-panel :deep(.el-table td){padding:10px 0}.table-panel small{display:block;margin-top:5px;color:#8995a8;font-size:11px;line-height:1.4}.reason-preview{max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#a16207!important}.table-panel .el-pagination{justify-content:flex-end;margin-top:13px}.detail-content{padding-bottom:28px}.detail-title{display:flex;justify-content:space-between;align-items:center;margin:2px 0 15px}.detail-title h2{font-size:19px;margin:0}.detail-title p{margin:6px 0 0;color:#64748b}.verdict-box{padding:12px 14px;margin-bottom:13px;border:1px solid #e2e8f0;border-radius:12px}.verdict-success{background:#f0fdf4;border-color:#bbf7d0}.verdict-warning{background:#fffbeb;border-color:#fde68a}.verdict-danger{background:#fef2f2;border-color:#fecaca}.verdict-top,.history-title{display:flex;justify-content:space-between;align-items:center;gap:8px;flex-wrap:wrap}.verdict-box p{margin:8px 0 0;color:#475569;font-size:13px;line-height:1.55}.url-text{overflow-wrap:anywhere}.history{margin-top:22px}.history h3,.rules-content h3{font-size:15px;margin:0 0 10px}.history p{margin:5px 0;color:#64748b;line-height:1.5}.history small{color:#8692a5;line-height:1.5}.action-bar{display:flex;gap:8px;flex-wrap:wrap;margin-top:20px;padding-top:15px;border-top:1px solid #e8edf4}.footnote,.rules-note{color:#78869a;font-size:12px;line-height:1.65}.rules-content{display:grid;gap:15px;padding-bottom:24px}.rules-content :deep(.el-alert){border-radius:11px}.allowlist{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;padding:13px;background:#f8fafc;border:1px solid #e2eaf4;border-radius:11px}.allowlist strong,.allowlist small{grid-column:1/-1}.allowlist span{padding:8px;background:#fff;border:1px solid #e5eaf1;border-radius:8px;color:#334155;font-size:12px}.allowlist small{color:#64748b;line-height:1.55}.test-tags{display:flex;flex-wrap:wrap;gap:7px}.rules-note{margin:0;padding:10px 12px;background:#f8fafc;border-radius:9px}@media(max-width:1250px){.summary-grid{grid-template-columns:repeat(3,minmax(0,1fr))}.decision-strip{padding-right:12px}.policy-link{position:static;transform:none;grid-column:1/-1;justify-self:end}}@media(max-width:760px){.page-heading{display:grid}.page-heading h1{font-size:23px}.heading-actions{width:100%}.heading-actions :deep(.el-button){flex:1}.decision-strip{grid-template-columns:1fr}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.filters{align-items:stretch}.filters .el-input,.filters .el-select{width:100%}.filters .el-button{flex:1}.table-panel{overflow:hidden;padding:6px}.table-panel :deep(.el-table){font-size:12px}.table-panel .el-pagination{justify-content:center;flex-wrap:wrap}.allowlist{grid-template-columns:1fr}.allowlist strong,.allowlist small{grid-column:auto}}
</style>
