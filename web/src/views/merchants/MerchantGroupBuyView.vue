<template>
  <div class="group-buy-page">
    <section class="group-buy-hero">
      <div>
        <div class="eyebrow">外部渠道导流 · 第一阶段</div>
        <h1>团购渠道</h1>
        <p>给门店配置抖音生活服务或美团团购链接，生成专属跳转码，并了解入口被打开的次数。</p>
      </div>
      <div class="hero-tags">
        <el-tag type="warning" effect="dark">仅统计跳转点击</el-tag>
        <el-tag effect="plain">不读取平台订单</el-tag>
      </div>
    </section>

    <el-alert class="scope-alert" type="info" :closable="false" show-icon>
      <template #title>这不是支付或订单对接</template>
      <template #default>
        顾客扫码后会跳转到你填写的官方平台链接。本页只累计跳转次数，不代表顾客已下单、付款或核销；不会记录顾客 IP、手机号或跨站标识。
      </template>
    </el-alert>

    <section class="store-toolbar">
      <div>
        <label for="group-buy-store">当前门店</label>
        <p>每家门店、每个平台分别配置，点击统计互不混用。</p>
      </div>
      <el-select id="group-buy-store" v-model="selectedStoreId" placeholder="选择门店" filterable style="width: 320px" :loading="storesLoading" :disabled="Boolean(savingChannel)" @change="loadLinks">
        <el-option v-for="store in stores" :key="store.id" :label="store.name" :value="store.id" />
      </el-select>
    </section>

    <section v-if="selectedStore" class="summary-grid">
      <article class="summary-card summary-primary">
        <span>当前门店</span>
        <strong>{{ selectedStore.name }}</strong>
        <small>配置按门店独立保存</small>
      </article>
      <article class="summary-card">
        <span>已配置渠道</span>
        <strong>{{ configuredCount }}<em> / 2</em></strong>
        <small>抖音生活服务、美团</small>
      </article>
      <article class="summary-card">
        <span>累计跳转次数</span>
        <strong>{{ totalClicks.toLocaleString('zh-CN') }}</strong>
        <small>含重复打开，不等于独立顾客数或成交</small>
      </article>
    </section>

    <section v-if="!storesLoading && stores.length === 0" class="empty-stores">
      <h2>先添加一家门店</h2>
      <p>团购渠道入口需要绑定到门店，避免多个门店的链接和点击统计混在一起。</p>
      <el-button type="primary" @click="$router.push('/merchant/stores')">去门店管理</el-button>
    </section>

    <section v-else-if="selectedStore" v-loading="linksLoading" class="channel-grid">
      <article v-for="channel in channels" :key="channel.key" class="channel-card">
        <header class="channel-heading">
          <div class="channel-logo" :class="channel.key">{{ channel.mark }}</div>
          <div class="channel-title">
            <h2>{{ channel.name }}</h2>
            <p>{{ channel.description }}</p>
          </div>
          <el-tag :type="entryStateType(card(channel.key))" effect="light">
            {{ !card(channel.key).code ? '待配置' : entryStateLabel(card(channel.key)) }}
          </el-tag>
        </header>

        <el-alert
          v-if="card(channel.key).code"
          class="review-alert"
          :type="reviewAlertType(card(channel.key))"
          :closable="false"
          show-icon
          :title="reviewAlertTitle(card(channel.key))"
          :description="reviewAlertDescription(card(channel.key))"
        />

        <div class="link-input-area">
          <label :for="`url-${channel.key}`">官方团购落地链接</label>
          <el-input
            :id="`url-${channel.key}`"
            v-model="card(channel.key).destination_url"
            type="textarea"
            :rows="3"
            maxlength="1000"
            show-word-limit
            :placeholder="channel.placeholder"
          />
          <small>仅接受 HTTPS 的{{ channel.name }}官方域名；系统不会访问、抓取或预览该外链。</small>
        </div>

        <div class="channel-actions">
          <el-switch v-model="card(channel.key).enabled" active-text="启用入口" inactive-text="暂停入口" :disabled="linksLoading || Boolean(savingChannel)" />
          <el-button type="primary" :loading="savingChannel === channel.key" :disabled="linksLoading || Boolean(savingChannel && savingChannel !== channel.key)" @click="save(channel.key)">
            {{ card(channel.key).code ? '保存配置' : '保存并生成二维码' }}
          </el-button>
        </div>

        <div v-if="card(channel.key).code" class="qr-result">
          <div class="qr-frame">
            <img v-if="qrImages[channel.key]" :src="qrImages[channel.key]" :alt="`${channel.name}专属跳转二维码`" />
            <div v-else class="qr-placeholder">正在生成二维码…</div>
          </div>
          <div class="qr-info">
            <span class="qr-label">门店专属跳转码</span>
            <strong>{{ Number(card(channel.key).click_count || 0).toLocaleString('zh-CN') }} 次</strong>
            <small>最近点击：{{ formatTime(card(channel.key).last_clicked_at) }}</small>
            <small class="qr-url">{{ redirectUrl(card(channel.key).code) }}</small>
            <div class="qr-actions">
              <el-button size="small" @click="copyLink(channel.key)">复制入口链接</el-button>
              <el-button size="small" @click="downloadQR(channel.key)">下载二维码</el-button>
            </div>
          </div>
        </div>
        <div v-else class="qr-empty">
          <span class="empty-qr-mark">↗</span>
          <span>保存平台链接后，这里会生成门店专属二维码。</span>
        </div>
      </article>
    </section>

    <footer v-if="selectedStore" class="phase-note">
      <strong>当前能力边界</strong>
      <span>平台端可对商家提交的外部链接做内部审核、拒绝或暂停；这不是抖音/美团的官方资质或 API 审核。只有在获得平台授权并明确接口范围后，才应另行评估订单、核销、退款或对账能力。当前点击统计不能代替订单或营业额。</span>
    </footer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { fetchMerchantGroupBuyLinks, fetchMerchantStores, saveMerchantGroupBuyLink } from '../../api/modules'

const channels = [
  {
    key: 'douyin',
    name: '抖音生活服务',
    mark: '抖',
    description: '适用于抖音团购商品或门店页面',
    placeholder: '粘贴抖音生活服务的 HTTPS 团购链接，例如 https://life.douyin.com/...'
  },
  {
    key: 'meituan',
    name: '美团 / 大众点评',
    mark: '美',
    description: '适用于美团或大众点评团购页面',
    placeholder: '粘贴美团或大众点评的 HTTPS 团购链接，例如 https://i.meituan.com/...'
  }
]

const stores = ref([])
const selectedStoreId = ref(null)
const storesLoading = ref(false)
const linksLoading = ref(false)
const savingChannel = ref('')
const qrImages = reactive({ douyin: '', meituan: '' })
const forms = reactive({
  douyin: { channel: 'douyin', destination_url: '', enabled: true, code: '', click_count: 0, last_clicked_at: null, review_status: 'pending', review_method: '', review_rule_id: '', review_reason: '', admin_suspended: false, suspension_reason: '' },
  meituan: { channel: 'meituan', destination_url: '', enabled: true, code: '', click_count: 0, last_clicked_at: null, review_status: 'pending', review_method: '', review_rule_id: '', review_reason: '', admin_suspended: false, suspension_reason: '' }
})

const selectedStore = computed(() => stores.value.find((item) => Number(item.id) === Number(selectedStoreId.value)))
const configuredCount = computed(() => channels.filter(({ key }) => Boolean(forms[key].code)).length)
const totalClicks = computed(() => channels.reduce((sum, { key }) => sum + Number(forms[key].click_count || 0), 0))
const card = (key) => forms[key]
const entryStateLabel = (item) => {
  if (item.admin_suspended) return '平台暂停'
  if (!item.enabled) return '商家已停用'
  if (item.review_status === 'pending') return '待审核'
  if (item.review_status === 'rejected') return '未通过审核'
  return item.review_status === 'approved' ? '审核通过，可跳转' : '待审核'
}
const entryStateType = (item) => item.review_status === 'approved' && item.enabled && !item.admin_suspended ? 'success' : item.review_status === 'rejected' || item.admin_suspended ? 'danger' : 'warning'
const reviewAlertType = (item) => item.admin_suspended || item.review_status === 'rejected' ? 'warning' : item.review_status === 'approved' ? 'success' : 'info'
const reviewMethodLabel = (item) => item.review_method === 'auto' ? '系统自动' : item.review_method === 'manual' ? '人工处理' : '旧记录'
const reviewRuleLabel = (item) => item.review_rule_id ? `规则 ${item.review_rule_id}` : '尚无规则记录'
const reviewAlertTitle = (item) => item.admin_suspended ? '平台已暂停此入口' : item.review_status === 'approved' ? `平台内部审核已通过 · ${reviewMethodLabel(item)}` : item.review_status === 'rejected' ? `平台内部审核未通过 · ${reviewMethodLabel(item)}` : item.review_method === 'manual' ? '待人工复核' : '等待平台内部审核'
const reviewAlertDescription = (item) => item.admin_suspended ? `${item.suspension_reason || '如需恢复，请联系平台运营。'} · ${reviewRuleLabel(item)}` : item.review_status === 'rejected' ? `${reviewRuleLabel(item)}：${item.review_reason || '请检查链接后重新提交。'} 修改链接并保存后会重新判定。` : item.review_status === 'approved' ? `${reviewRuleLabel(item)}：${item.enabled ? '入口当前允许跳转' : '审核通过，但商家已停用入口'}；点击量不代表订单或收入。` : `${reviewRuleLabel(item)}：${item.review_reason || '审核通过前，专属入口不会跳转到外部平台。'}`

const makeQR = async (key) => {
  const code = forms[key].code
  if (!code) {
    qrImages[key] = ''
    return
  }
  qrImages[key] = await QRCode.toDataURL(redirectUrl(code), {
    width: 220,
    margin: 2,
    color: { dark: '#12304a', light: '#ffffff' },
    errorCorrectionLevel: 'M'
  })
}

const redirectUrl = (code) => `${window.location.origin}/api/v1/group-buy/r/${encodeURIComponent(code)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '暂无'

const loadStores = async () => {
  storesLoading.value = true
  try {
    const res = await fetchMerchantStores({ page: 1, page_size: 100 })
    stores.value = res.data?.list || []
    if (stores.value.length) {
      selectedStoreId.value = stores.value[0].id
      await loadLinks()
    }
  } catch {
    ElMessage.error('读取门店失败，请稍后重试')
  } finally {
    storesLoading.value = false
  }
}

const loadLinks = async () => {
  if (!selectedStoreId.value) return
  const requestStoreId = selectedStoreId.value
  linksLoading.value = true
  Object.assign(forms.douyin, { destination_url: '', enabled: true, code: '', click_count: 0, last_clicked_at: null, review_status: 'pending', review_method: '', review_rule_id: '', review_reason: '', admin_suspended: false, suspension_reason: '' })
  Object.assign(forms.meituan, { destination_url: '', enabled: true, code: '', click_count: 0, last_clicked_at: null, review_status: 'pending', review_method: '', review_rule_id: '', review_reason: '', admin_suspended: false, suspension_reason: '' })
  qrImages.douyin = ''
  qrImages.meituan = ''
  try {
    const res = await fetchMerchantGroupBuyLinks(requestStoreId)
    if (Number(selectedStoreId.value) !== Number(requestStoreId)) return
    for (const item of (res.data?.links || [])) {
      if (!forms[item.channel]) continue
      Object.assign(forms[item.channel], item)
      await makeQR(item.channel)
    }
  } catch {
    if (Number(selectedStoreId.value) === Number(requestStoreId)) ElMessage.error('读取团购渠道配置失败')
  } finally {
    if (Number(selectedStoreId.value) === Number(requestStoreId)) linksLoading.value = false
  }
}

const save = async (key) => {
  const destination = forms[key].destination_url.trim()
  if (!selectedStoreId.value) {
    ElMessage.warning('请先选择门店')
    return
  }
  if (!destination) {
    ElMessage.warning('请先填写平台团购链接')
    return
  }
  savingChannel.value = key
  try {
    const res = await saveMerchantGroupBuyLink({
      store_id: Number(selectedStoreId.value),
      channel: key,
      destination_url: destination,
      enabled: Boolean(forms[key].enabled)
    })
    Object.assign(forms[key], res.data?.link || {})
    await makeQR(key)
    const saved = res.data?.link || {}
    if (saved.review_status === 'rejected') ElMessage.warning(`链接已保存但未通过：${saved.review_rule_id || ''} ${saved.review_reason || '请修改链接后重试'}`)
    else if (saved.review_status === 'pending') ElMessage.warning(`配置已保存，等待人工核验：${saved.review_rule_id || ''} ${saved.review_reason || ''}`)
    else if (saved.review_status === 'approved' && saved.review_method === 'auto') ElMessage.success(`系统规则自动通过（${saved.review_rule_id || '规则记录缺失'}），专属二维码已更新`)
    else if (saved.review_status === 'approved' && saved.review_method === 'manual') ElMessage.success(`人工审核通过（${saved.review_rule_id || '规则记录缺失'}），专属二维码已更新`)
    else ElMessage.success('配置已保存；该历史入口的审核来源未记录')
  } catch (error) {
    ElMessage.error(error.response?.data?.message || '保存失败，请检查链接是否为该平台官方 HTTPS 地址')
  } finally {
    savingChannel.value = ''
  }
}

const copyLink = async (key) => {
  try {
    await navigator.clipboard.writeText(redirectUrl(forms[key].code))
    ElMessage.success('入口链接已复制')
  } catch {
    ElMessage.warning('当前浏览器不支持自动复制，请使用下载二维码')
  }
}

const downloadQR = (key) => {
  if (!qrImages[key]) return
  const anchor = document.createElement('a')
  anchor.href = qrImages[key]
  anchor.download = `${selectedStore.value?.name || '门店'}-${channels.find((item) => item.key === key)?.name || '团购'}-跳转码.png`
  anchor.click()
}

onMounted(loadStores)
</script>

<style scoped>
.group-buy-page { display: grid; gap: 18px; color: #102338; }
.group-buy-hero { display: flex; justify-content: space-between; align-items: flex-start; gap: 22px; padding: 28px 30px; border-radius: 22px; background: linear-gradient(115deg, #0e2944 0%, #174b63 57%, #1b7181 100%); color: #fff; box-shadow: 0 18px 40px rgba(15, 43, 67, .14); }
.eyebrow { color: #a5e7e2; font-size: 12px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
.group-buy-hero h1 { margin: 8px 0 7px; font-size: 30px; line-height: 1.2; }
.group-buy-hero p { margin: 0; max-width: 700px; color: #d2e6ee; line-height: 1.7; }
.hero-tags { display: flex; flex-wrap: wrap; gap: 9px; justify-content: flex-end; }
.scope-alert { border-radius: 15px; }
.scope-alert :deep(.el-alert__title) { font-weight: 800; }
.scope-alert :deep(.el-alert__description) { line-height: 1.75; }
.review-alert { margin-top: 14px; border-radius: 12px; }
.store-toolbar { display: flex; justify-content: space-between; align-items: center; gap: 18px; padding: 18px 22px; background: #fff; border: 1px solid #e4edf2; border-radius: 16px; }
.store-toolbar label { font-size: 15px; font-weight: 800; }
.store-toolbar p { margin: 5px 0 0; color: #6b7d8d; font-size: 13px; }
.summary-grid { display: grid; grid-template-columns: 1.25fr 1fr 1fr; gap: 14px; }
.summary-card { display: grid; gap: 6px; min-height: 104px; padding: 17px 19px; border: 1px solid #e3edf2; border-radius: 16px; background: #fff; }
.summary-card span { color: #718393; font-size: 12px; font-weight: 700; }
.summary-card strong { font-size: 25px; line-height: 1.15; }
.summary-card strong em { color: #8997a3; font-size: 15px; font-style: normal; }
.summary-card small { color: #82909b; font-size: 11px; }
.summary-primary { background: #edf8f8; border-color: #cce9e8; }
.summary-primary strong { font-size: 21px; }
.channel-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.channel-card { min-width: 0; padding: 21px; border: 1px solid #e2ebf0; border-radius: 18px; background: #fff; box-shadow: 0 8px 22px rgba(20, 47, 67, .035); }
.channel-heading { display: flex; align-items: center; gap: 12px; }
.channel-logo { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 13px; font-size: 20px; font-weight: 900; }
.channel-logo.douyin { color: #fff; background: linear-gradient(135deg, #102634, #405260); }
.channel-logo.meituan { color: #fff; background: linear-gradient(135deg, #f29b38, #f2722e); }
.channel-title { flex: 1; min-width: 0; }
.channel-title h2 { margin: 0; font-size: 17px; }
.channel-title p { margin: 4px 0 0; color: #7c8995; font-size: 12px; }
.link-input-area { display: grid; gap: 8px; margin-top: 22px; }
.link-input-area label { font-size: 13px; font-weight: 800; }
.link-input-area small { color: #82909b; line-height: 1.55; }
.channel-actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-top: 17px; }
.qr-result { display: flex; align-items: center; gap: 17px; margin-top: 19px; padding: 14px; border-radius: 14px; background: #f6fafb; border: 1px solid #e8f0f2; }
.qr-frame { flex: 0 0 132px; display: grid; place-items: center; width: 132px; height: 132px; padding: 7px; border-radius: 12px; background: #fff; border: 1px solid #e2ebee; }
.qr-frame img { width: 100%; height: 100%; object-fit: contain; }
.qr-placeholder { color: #8a98a2; font-size: 11px; text-align: center; }
.qr-info { display: grid; gap: 5px; min-width: 0; }
.qr-label { color: #68808c; font-size: 11px; font-weight: 700; }
.qr-info strong { font-size: 22px; }
.qr-info small { color: #7b8b96; font-size: 11px; }
.qr-info .qr-url { overflow-wrap: anywhere; color: #35707e; }
.qr-actions { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 4px; }
.qr-empty { display: flex; align-items: center; gap: 10px; margin-top: 19px; padding: 16px; border: 1px dashed #cfdee4; border-radius: 13px; color: #7b8b96; font-size: 12px; }
.empty-qr-mark { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; color: #257582; background: #e7f5f3; font-size: 19px; font-weight: 900; }
.empty-stores { display: grid; justify-items: start; gap: 10px; padding: 36px; border-radius: 18px; background: #fff; }
.empty-stores h2, .empty-stores p { margin: 0; }
.empty-stores p { color: #718393; }
.phase-note { display: flex; gap: 12px; padding: 14px 17px; border-radius: 13px; background: #f1f6fa; color: #617585; font-size: 12px; line-height: 1.7; }
.phase-note strong { flex: 0 0 auto; color: #23445a; }
@media (max-width: 1100px) { .channel-grid { grid-template-columns: 1fr; } }
@media (max-width: 700px) { .group-buy-hero, .store-toolbar { flex-direction: column; align-items: stretch; } .hero-tags { justify-content: flex-start; } .summary-grid { grid-template-columns: 1fr; } .qr-result { align-items: flex-start; flex-direction: column; } .qr-frame { align-self: center; } .phase-note { flex-direction: column; gap: 4px; } }
</style>
