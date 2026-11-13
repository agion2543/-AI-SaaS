<template>
  <div class="group-buy-mobile">
    <van-nav-bar
      title="团购渠道"
      left-arrow
      fixed
      placeholder
      @click-left="router.push('/m/merchant/home')"
      @click-right="loadPage"
    >
      <template #right>
        <span class="refresh-action"><van-icon name="replay" />刷新</span>
      </template>
    </van-nav-bar>

    <main class="page-content">
      <section class="hero-card">
        <div class="hero-kicker">门店引流 · 外部平台</div>
        <h1>把团购入口放进门店</h1>
        <p>分别配置抖音生活服务、美团或大众点评的官方团购链接，为每家门店生成专属跳转码。</p>
        <div class="hero-pills">
          <span>门店独立配置</span>
          <span>审核后方可跳转</span>
        </div>
      </section>

      <section class="scope-note" aria-label="数据范围说明">
        <div class="note-icon">i</div>
        <div>
          <strong>点击不等于订单或收入</strong>
          <p>这里只累计专属入口被打开的次数，重复打开会重复计数；不代表下单、付款或核销，也不记录顾客 IP、手机号或跨站标识。</p>
        </div>
      </section>

      <div v-if="accessState === 'expired'" class="state-banner expired-banner" role="status">
        <van-icon name="warning-o" />
        <span>订阅已到期：现有门店和团购数据可只读查看，不能新建或修改配置。</span>
      </div>

      <section v-if="pageLoading" class="loading-panel">
        <van-loading type="spinner" color="#168f8c">正在读取门店与团购入口…</van-loading>
      </section>

      <section v-else-if="pageError" class="state-panel error-panel">
        <van-icon name="warning-o" />
        <h2>暂时无法读取团购渠道</h2>
        <p>{{ pageError }}</p>
        <van-button type="primary" plain round @click="loadPage">重试</van-button>
      </section>

      <section v-else-if="blockedMessage" class="state-panel blocked-panel">
        <div class="blocked-mark"><van-icon :name="accessState === 'paused' || merchant.status === 'suspended' ? 'pause-circle-o' : 'lock'" /></div>
        <h2>{{ blockedTitle }}</h2>
        <p>{{ blockedMessage }}</p>
        <van-button type="primary" round @click="openSubscription">查看订阅状态</van-button>
        <van-button plain round @click="router.push('/m/merchant/home')">返回工作台</van-button>
      </section>

      <template v-else>
        <section v-if="stores.length" class="store-section">
          <div class="section-heading">
            <div>
              <h2>选择门店</h2>
              <p>渠道链接和点击数按门店分别统计</p>
            </div>
            <span class="store-count">{{ stores.length }} 家</span>
          </div>
          <div class="store-select-wrap">
            <van-icon name="shop-o" />
            <select v-model="selectedStoreId" aria-label="选择门店" :disabled="linksLoading || Boolean(savingChannel)" @change="loadLinks">
              <option v-for="store in stores" :key="store.id" :value="String(store.id)">{{ store.name || `门店 ${store.id}` }}</option>
            </select>
            <van-icon class="select-arrow" name="arrow-down" />
          </div>

          <div v-if="selectedStore" class="summary-grid">
            <div class="summary-card store-summary">
              <span>当前门店</span>
              <strong>{{ selectedStore.name || `门店 ${selectedStore.id}` }}</strong>
            </div>
            <div class="summary-card">
              <span>已配置渠道</span>
              <strong>{{ configuredCount }}<small> / 2</small></strong>
            </div>
            <div class="summary-card">
              <span>累计跳转点击</span>
              <strong>{{ totalClicks.toLocaleString('zh-CN') }}</strong>
            </div>
          </div>
        </section>

        <section v-if="linksLoading" class="loading-panel compact-loading">
          <van-loading type="spinner" color="#168f8c">正在读取该门店的渠道…</van-loading>
        </section>

        <section v-else-if="stores.length === 0" class="state-panel empty-panel">
          <div class="empty-mark"><van-icon name="shop-o" /></div>
          <h2>{{ readOnly ? '暂无可查看门店' : '先添加一家门店' }}</h2>
          <p>{{ readOnly ? '当前账号没有可读取的门店；订阅到期后不能新增或修改门店。' : '团购入口需要绑定门店，避免不同门店的链接和点击统计混在一起。' }}</p>
          <van-button v-if="!readOnly" type="primary" round @click="router.push('/m/merchant/stores')">前往门店管理</van-button>
        </section>

        <section v-else class="channel-list" aria-label="团购渠道配置">
          <article v-for="channel in channels" :key="channel.key" class="channel-card">
            <header class="channel-heading">
              <div class="channel-mark" :class="channel.key">{{ channel.mark }}</div>
              <div class="channel-title">
                <h2>{{ channel.name }}</h2>
                <p>{{ channel.description }}</p>
              </div>
              <van-tag round :type="entryStateType(card(channel.key))" class="entry-tag">{{ entryStateLabel(card(channel.key)) }}</van-tag>
            </header>

            <div v-if="card(channel.key).code" class="review-box" :class="`review-${reviewTone(card(channel.key))}`">
              <div class="review-title">
                <van-icon :name="reviewIcon(card(channel.key))" />
                <strong>{{ reviewTitle(card(channel.key)) }}</strong>
              </div>
              <p>{{ reviewDescription(card(channel.key)) }}</p>
              <div v-if="card(channel.key).review_rule_id" class="rule-id">规则 {{ card(channel.key).review_rule_id }}</div>
            </div>

            <div class="field-section">
              <label :for="`mobile-group-buy-${channel.key}`">官方团购落地链接</label>
              <van-field
                :id="`mobile-group-buy-${channel.key}`"
                v-model="card(channel.key).destination_url"
                type="textarea"
                rows="2"
                autosize
                maxlength="1000"
                show-word-limit
                :readonly="readOnly"
                :placeholder="channel.placeholder"
                class="url-field"
              />
              <p class="field-hint">仅接受该平台官方 HTTPS 域名。系统会按现有安全规则校验，不会访问、抓取或预览外链。</p>
            </div>

            <div class="enable-row">
              <div>
                <strong>{{ card(channel.key).enabled ? '入口已启用' : '入口已暂停' }}</strong>
                <span>关闭后专属跳转入口不可用</span>
              </div>
              <van-switch v-model="card(channel.key).enabled" :disabled="readOnly || linksLoading || Boolean(savingChannel)" size="22" aria-label="启用或暂停团购入口" />
            </div>

            <van-button
              v-if="!readOnly"
              type="primary"
              block
              round
              class="save-button"
              :loading="savingChannel === channel.key"
              :disabled="linksLoading || Boolean(savingChannel) || !card(channel.key).destination_url.trim()"
              @click="save(channel.key)"
            >
              {{ card(channel.key).code ? '保存配置' : '保存链接并生成跳转码' }}
            </van-button>

            <section v-if="card(channel.key).code" class="qr-panel" :aria-label="`${channel.name}跳转入口`">
              <div class="qr-heading">
                <div>
                  <h3>门店专属跳转码</h3>
                  <p>扫码后先经过本系统审核与启用状态检查</p>
                </div>
                <span class="click-count"><strong>{{ Number(card(channel.key).click_count || 0).toLocaleString('zh-CN') }}</strong> 次点击</span>
              </div>
              <div class="qr-image-wrap">
                <img v-if="qrImages[channel.key]" :src="qrImages[channel.key]" :alt="`${channel.name}门店专属跳转二维码`" />
                <van-loading v-else type="spinner" color="#168f8c">二维码生成中</van-loading>
              </div>
              <p class="last-clicked">最近点击：{{ formatTime(card(channel.key).last_clicked_at) }}</p>
              <code class="redirect-url">{{ redirectUrl(card(channel.key).code) }}</code>
              <div class="qr-actions">
                <van-button plain round icon="description" @click="copyLink(channel.key)">复制入口链接</van-button>
                <van-button plain round icon="down" :disabled="!qrImages[channel.key]" @click="downloadQR(channel.key)">下载二维码</van-button>
              </div>
              <p class="click-disclaimer">统计的是跳转点击，不是独立顾客数、订单或收入。</p>
            </section>

            <div v-else class="qr-empty">
              <van-icon name="qr" />
              <span>保存平台链接后，将生成该门店的专属跳转二维码。</span>
            </div>
          </article>
        </section>

        <footer v-if="selectedStore" class="capability-note">
          <strong>功能范围</strong>
          <p>当前只管理抖音、美团/大众点评官方链接，并统计跳转点击；不接入外部平台订单、支付、核销或收入数据。商家端目前提供当前审核状态与原因，不提供逐条审核历史。</p>
        </footer>
      </template>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import QRCode from 'qrcode'
import { fetchMerchantGroupBuyLinks, fetchMerchantStores, fetchMerchantSubscription, saveMerchantGroupBuyLink } from '@/api/modules'
import { merchantSubscriptionAccessState } from '@/utils/merchantSubscriptionState'

const router = useRouter()
const channels = [
  {
    key: 'douyin',
    name: '抖音生活服务',
    mark: '抖',
    description: '抖音团购商品或门店页面',
    placeholder: '粘贴抖音生活服务官方 HTTPS 团购链接'
  },
  {
    key: 'meituan',
    name: '美团 / 大众点评',
    mark: '美',
    description: '美团或大众点评团购页面',
    placeholder: '粘贴美团或大众点评官方 HTTPS 团购链接'
  }
]

const makeDefaultForm = () => ({
  channel: '', destination_url: '', enabled: true, code: '', click_count: 0,
  last_clicked_at: null, review_status: 'pending', review_method: '',
  review_rule_id: '', review_reason: '', admin_suspended: false,
  suspension_reason: ''
})
const stores = ref([])
const selectedStoreId = ref('')
const merchant = ref({})
const accessState = ref('checking')
const pageLoading = ref(false)
const linksLoading = ref(false)
const pageError = ref('')
const savingChannel = ref('')
const qrImages = reactive({ douyin: '', meituan: '' })
const forms = reactive({ douyin: makeDefaultForm(), meituan: makeDefaultForm() })

const selectedStore = computed(() => stores.value.find((store) => String(store.id) === String(selectedStoreId.value)))
const readOnly = computed(() => accessState.value === 'expired')
const blockedMessage = computed(() => {
  if (accessState.value === 'paused') return merchant.value.subscription_pause_reason || '平台已暂停商家业务，暂停期间不能查看或操作团购经营数据。'
  if (merchant.value.status === 'suspended') return '商家业务已被平台暂停，暂停期间不能查看或操作团购经营数据。'
  if (accessState.value === 'unavailable') return '当前订阅未开通或状态不可用，暂时不能查看团购经营数据。'
  return ''
})
const blockedTitle = computed(() => accessState.value === 'paused' || merchant.value.status === 'suspended' ? '商家业务已暂停' : '订阅状态不可用')
const configuredCount = computed(() => channels.filter(({ key }) => Boolean(forms[key].code)).length)
const totalClicks = computed(() => channels.reduce((sum, { key }) => sum + Number(forms[key].click_count || 0), 0))
const card = (key) => forms[key]

const rowsFrom = (data) => {
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.list)) return data.list
  if (Array.isArray(data?.items)) return data.items
  return []
}
const entryStateLabel = (item) => {
  if (item.admin_suspended) return '平台已暂停'
  if (!item.code) return '待配置'
  if (!item.enabled) return '商家已停用'
  if (item.review_status === 'pending') return '待审核'
  if (item.review_status === 'rejected') return '未通过审核'
  if (item.review_status === 'approved') return '审核通过'
  return '待审核'
}
const entryStateType = (item) => {
  if (item.admin_suspended || item.review_status === 'rejected') return 'danger'
  if (item.review_status === 'approved' && item.enabled) return 'success'
  return 'warning'
}
const methodLabel = (item) => item.review_method === 'auto' ? '系统自动' : item.review_method === 'manual' ? '人工处理' : '历史记录'
const reviewTone = (item) => item.admin_suspended || item.review_status === 'rejected' ? 'warning' : item.review_status === 'approved' ? 'success' : 'info'
const reviewIcon = (item) => item.admin_suspended || item.review_status === 'rejected' ? 'warning-o' : item.review_status === 'approved' ? 'checked' : 'clock-o'
const reviewTitle = (item) => {
  if (item.admin_suspended) return '平台已暂停此入口'
  if (item.review_status === 'approved') return `平台内部审核已通过 · ${methodLabel(item)}`
  if (item.review_status === 'rejected') return `平台内部审核未通过 · ${methodLabel(item)}`
  return item.review_method === 'manual' ? '等待人工复核' : '等待平台内部审核'
}
const reviewDescription = (item) => {
  if (item.admin_suspended) return `${item.suspension_reason || '入口暂不可跳转，如需恢复请联系平台运营。'} 商家无法自行解除暂停。`
  if (item.review_status === 'rejected') return `${item.review_reason || '请检查链接后修改并保存。'} 修改链接后会按现有规则重新判定。`
  if (item.review_status === 'approved') return item.enabled ? '当前入口可按审核及门店状态跳转；点击量不代表订单或收入。' : '审核通过，但商家已停用此入口。'
  return item.review_reason || '审核通过前，专属入口不会跳转到外部平台。'
}
const redirectUrl = (code) => `${window.location.origin}/api/v1/group-buy/r/${encodeURIComponent(code)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '暂无'

const resetForms = () => {
  for (const { key } of channels) {
    Object.assign(forms[key], makeDefaultForm(), { channel: key })
    qrImages[key] = ''
  }
}

const makeQR = async (key) => {
  const code = forms[key].code
  qrImages[key] = code ? await QRCode.toDataURL(redirectUrl(code), {
    width: 240,
    margin: 2,
    color: { dark: '#123247', light: '#ffffff' },
    errorCorrectionLevel: 'M'
  }) : ''
}

const loadLinks = async () => {
  if (!selectedStoreId.value || !['active', 'expired'].includes(accessState.value)) return
  const requestStoreId = String(selectedStoreId.value)
  linksLoading.value = true
  pageError.value = ''
  resetForms()
  try {
    const response = await fetchMerchantGroupBuyLinks(Number(requestStoreId))
    if (String(selectedStoreId.value) !== requestStoreId) return
    for (const item of response.data?.links || []) {
      if (!forms[item.channel]) continue
      Object.assign(forms[item.channel], item)
      try {
        await makeQR(item.channel)
      } catch {
        qrImages[item.channel] = ''
      }
    }
  } catch (error) {
    if (String(selectedStoreId.value) === requestStoreId) {
      const message = error.response?.data?.message || '读取团购渠道配置失败，请稍后重试。'
      if (message.includes('订阅已被平台暂停')) accessState.value = 'paused'
      else if (message.includes('订阅当前不可用')) accessState.value = 'unavailable'
      else pageError.value = message
    }
  } finally {
    if (String(selectedStoreId.value) === requestStoreId) linksLoading.value = false
  }
}

const loadStores = async () => {
  const response = await fetchMerchantStores({ page: 1, page_size: 100 })
  stores.value = rowsFrom(response.data)
  if (stores.value.length) {
    selectedStoreId.value = String(stores.value[0].id)
    await loadLinks()
  }
}

const loadPage = async () => {
  if (pageLoading.value) return
  pageLoading.value = true
  pageError.value = ''
  stores.value = []
  selectedStoreId.value = ''
  resetForms()
  accessState.value = 'checking'
  try {
    const subscriptionResponse = await fetchMerchantSubscription()
    merchant.value = subscriptionResponse.data?.merchant || {}
    accessState.value = merchantSubscriptionAccessState(merchant.value)
    if (merchant.value.status === 'suspended' || !['active', 'expired'].includes(accessState.value)) return
    await loadStores()
  } catch (error) {
    const message = error.response?.data?.message || '读取商家或门店状态失败，请稍后重试。'
    if (message.includes('订阅已被平台暂停')) accessState.value = 'paused'
    else if (message.includes('订阅当前不可用')) accessState.value = 'unavailable'
    else pageError.value = message
  } finally {
    pageLoading.value = false
  }
}

const save = async (key) => {
  if (accessState.value !== 'active') {
    showToast('订阅到期后只能查看，不能修改配置')
    return
  }
  const destination = forms[key].destination_url.trim()
  if (!destination || !selectedStoreId.value) {
    showToast('请先选择门店并填写官方团购链接')
    return
  }
  savingChannel.value = key
  try {
    const response = await saveMerchantGroupBuyLink({
      store_id: Number(selectedStoreId.value),
      channel: key,
      destination_url: destination,
      enabled: Boolean(forms[key].enabled)
    })
    Object.assign(forms[key], response.data?.link || {})
    try {
      await makeQR(key)
    } catch {
      qrImages[key] = ''
    }
    const saved = response.data?.link || {}
    if (saved.admin_suspended) showToast('配置已保存，但入口仍处于平台暂停状态')
    else if (saved.review_status === 'rejected') showToast(`链接未通过：${saved.review_reason || '请修改后重试'}`)
    else if (saved.review_status === 'pending') showToast('配置已保存，等待审核完成后方可跳转')
    else showToast('配置已保存')
  } catch (error) {
    showToast(error.response?.data?.message || '保存失败，请检查链接或稍后重试')
  } finally {
    savingChannel.value = ''
  }
}

const copyLink = async (key) => {
  const url = redirectUrl(forms[key].code)
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(url)
    } else {
      const field = document.createElement('textarea')
      field.value = url
      field.setAttribute('readonly', '')
      field.style.position = 'fixed'
      field.style.opacity = '0'
      document.body.appendChild(field)
      field.select()
      const copied = document.execCommand('copy')
      field.remove()
      if (!copied) throw new Error('copy unavailable')
    }
    showToast('入口链接已复制')
  } catch {
    showToast('当前浏览器不支持自动复制，请下载二维码使用')
  }
}

const downloadQR = (key) => {
  if (!qrImages[key]) return
  const anchor = document.createElement('a')
  anchor.href = qrImages[key]
  anchor.download = `${selectedStore.value?.name || '门店'}-${channels.find((item) => item.key === key)?.name || '团购'}-跳转码.png`
  anchor.click()
}

const openSubscription = () => {
  router.push('/m/merchant/subscription')
}

onMounted(loadPage)
</script>

<style scoped>
.group-buy-mobile { min-height: 100vh; background: #f4f7f8; color: #18313c; }
.refresh-action { display: inline-flex; align-items: center; gap: 5px; color: #39737a; font-size: 13px; }
.page-content { display: grid; gap: 14px; padding: 14px 14px calc(96px + env(safe-area-inset-bottom)); }
.hero-card { position: relative; overflow: hidden; padding: 22px 20px 20px; border-radius: 20px; color: #fff; background: linear-gradient(140deg, #102e43 0%, #145966 58%, #168c88 100%); box-shadow: 0 12px 26px rgba(19, 70, 82, .17); }
.hero-card::after { position: absolute; right: -38px; bottom: -72px; width: 190px; height: 190px; border: 1px solid rgba(214, 255, 246, .16); border-radius: 50%; content: ''; }
.hero-kicker { color: #a9e7dd; font-size: 11px; font-weight: 800; letter-spacing: .12em; }
.hero-card h1 { position: relative; z-index: 1; margin: 9px 0 7px; font-size: 23px; line-height: 1.3; letter-spacing: -.02em; }
.hero-card p { position: relative; z-index: 1; max-width: 360px; margin: 0; color: #d5ecec; font-size: 13px; line-height: 1.65; }
.hero-pills { position: relative; z-index: 1; display: flex; flex-wrap: wrap; gap: 7px; margin-top: 15px; }
.hero-pills span { padding: 5px 9px; border: 1px solid rgba(218, 255, 249, .24); border-radius: 999px; color: #e4fbf7; background: rgba(255, 255, 255, .08); font-size: 11px; }
.scope-note { display: flex; align-items: flex-start; gap: 11px; padding: 14px; border: 1px solid #cfe9e5; border-radius: 16px; background: #f0fbf8; }
.note-icon { display: grid; flex: 0 0 24px; place-items: center; width: 24px; height: 24px; border-radius: 50%; color: #fff; background: #168f86; font-size: 14px; font-weight: 800; }
.scope-note strong { display: block; color: #126b64; font-size: 14px; }
.scope-note p { margin: 4px 0 0; color: #55736f; font-size: 12px; line-height: 1.6; }
.state-banner { display: flex; align-items: flex-start; gap: 8px; padding: 11px 12px; border-radius: 12px; font-size: 12px; line-height: 1.5; }
.expired-banner { color: #8a5b13; background: #fff7e8; border: 1px solid #f3dfb8; }
.expired-banner :deep(.van-icon) { margin-top: 1px; font-size: 16px; }
.loading-panel, .state-panel { display: grid; justify-items: center; gap: 9px; padding: 26px 20px; border: 1px solid #e4ecee; border-radius: 18px; background: #fff; text-align: center; }
.loading-panel { min-height: 110px; place-content: center; }
.compact-loading { padding: 17px; }
.state-panel h2 { margin: 3px 0 0; font-size: 17px; }
.state-panel p { max-width: 340px; margin: 0; color: #72858d; font-size: 13px; line-height: 1.6; }
.state-panel :deep(.van-button) { min-width: 154px; margin-top: 5px; }
.error-panel { color: #804c37; }
.error-panel > .van-icon { color: #df8c5d; font-size: 26px; }
.blocked-panel { padding-block: 28px; }
.blocked-mark, .empty-mark { display: grid; place-items: center; width: 56px; height: 56px; border-radius: 18px; color: #9d7628; background: #fff5da; font-size: 27px; }
.empty-mark { color: #168f8c; background: #e6f5f1; }
.blocked-panel :deep(.van-button + .van-button) { margin-top: 0; }
.store-section, .channel-card, .capability-note { padding: 16px; border: 1px solid #e4ecee; border-radius: 18px; background: #fff; box-shadow: 0 5px 18px rgba(26, 55, 67, .035); }
.section-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 12px; }
.section-heading h2 { margin: 0; font-size: 16px; }
.section-heading p { margin: 3px 0 0; color: #829198; font-size: 11px; }
.store-count { padding: 4px 8px; border-radius: 999px; color: #477579; background: #eff7f6; font-size: 11px; }
.store-select-wrap { position: relative; display: flex; align-items: center; gap: 9px; min-height: 48px; padding: 0 12px; border: 1px solid #dbe7e9; border-radius: 13px; color: #38817d; background: #fbfdfd; }
.store-select-wrap > .van-icon { flex: 0 0 auto; font-size: 19px; }
.store-select-wrap select { width: 100%; min-width: 0; height: 46px; padding: 0 22px 0 0; border: 0; outline: 0; color: #233a44; background: transparent; font: inherit; font-size: 14px; appearance: none; }
.store-select-wrap select:disabled { opacity: .6; }
.store-select-wrap .select-arrow { position: absolute; right: 12px; color: #93a5aa; font-size: 12px; pointer-events: none; }
.summary-grid { display: grid; grid-template-columns: 1.15fr .85fr; gap: 8px; margin-top: 10px; }
.summary-card { display: grid; align-content: center; gap: 3px; min-width: 0; min-height: 62px; padding: 9px 11px; border-radius: 12px; background: #f6f9f9; }
.summary-card span { color: #809197; font-size: 10px; }
.summary-card strong { overflow: hidden; color: #213b45; font-size: 17px; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; }
.summary-card small { color: #8b9ba0; font-size: 11px; font-weight: 500; }
.store-summary { grid-column: 1 / -1; min-height: 52px; background: #f0f8f7; }
.store-summary strong { color: #256d69; font-size: 15px; }
.channel-list { display: grid; gap: 13px; }
.channel-card { min-width: 0; padding: 15px; }
.channel-heading { display: flex; align-items: center; gap: 10px; min-width: 0; }
.channel-mark { display: grid; flex: 0 0 42px; place-items: center; width: 42px; height: 42px; border-radius: 14px; color: white; background: linear-gradient(135deg, #162e3b, #455a63); font-size: 19px; font-weight: 900; }
.channel-mark.meituan { background: linear-gradient(135deg, #f2a042, #ee7838); }
.channel-title { flex: 1; min-width: 0; }
.channel-title h2 { margin: 0; font-size: 15px; line-height: 1.3; }
.channel-title p { margin: 3px 0 0; color: #829198; font-size: 10px; line-height: 1.4; }
.entry-tag { flex: 0 0 auto; max-width: 92px; white-space: nowrap; }
.review-box { margin-top: 13px; padding: 11px 12px; border: 1px solid transparent; border-radius: 13px; }
.review-info { color: #446b80; background: #f1f8fc; border-color: #d8eaf4; }
.review-success { color: #35715e; background: #f0faf4; border-color: #d6edde; }
.review-warning { color: #8d6828; background: #fff8ec; border-color: #f1e2c5; }
.review-title { display: flex; align-items: center; gap: 6px; font-size: 12px; }
.review-title :deep(.van-icon) { font-size: 15px; }
.review-box p { margin: 5px 0 0; font-size: 11px; line-height: 1.55; }
.rule-id { margin-top: 4px; opacity: .75; font-size: 10px; }
.field-section { display: grid; gap: 7px; margin-top: 15px; }
.field-section label { color: #314a54; font-size: 12px; font-weight: 800; }
.url-field { overflow: hidden; padding: 0; border: 1px solid #e0e9eb; border-radius: 12px; background: #fcfdfd; }
.url-field :deep(.van-field__control) { min-height: 64px; padding: 10px 11px; color: #26424a; font-family: inherit; font-size: 12px; line-height: 1.55; overflow-wrap: anywhere; }
.url-field :deep(.van-field__word-limit) { padding: 0 10px 7px; color: #9aa9ad; font-size: 10px; }
.url-field :deep(.van-field__control:read-only) { color: #687d83; }
.field-hint { margin: 0; color: #87979c; font-size: 10px; line-height: 1.55; }
.enable-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 58px; margin-top: 11px; padding: 9px 1px; border-bottom: 1px solid #edf1f2; }
.enable-row > div { display: grid; gap: 2px; }
.enable-row strong { color: #3b555d; font-size: 12px; }
.enable-row span { color: #8c9a9f; font-size: 10px; }
.save-button { margin-top: 12px; min-height: 46px; font-weight: 700; }
.qr-panel { display: grid; justify-items: center; margin-top: 14px; padding: 14px 11px 12px; border: 1px solid #e5eeee; border-radius: 15px; background: #f7fbfa; }
.qr-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; width: 100%; }
.qr-heading h3 { margin: 0; color: #355660; font-size: 12px; }
.qr-heading p { margin: 3px 0 0; color: #8a9a9d; font-size: 9px; line-height: 1.4; }
.click-count { flex: 0 0 auto; padding: 5px 8px; border-radius: 999px; color: #28756c; background: #e7f5ef; font-size: 10px; }
.click-count strong { font-size: 14px; }
.qr-image-wrap { display: grid; place-items: center; width: 172px; height: 172px; margin-top: 12px; padding: 8px; border: 1px solid #e4ecee; border-radius: 14px; background: white; }
.qr-image-wrap img { display: block; width: 100%; height: 100%; object-fit: contain; }
.last-clicked { width: 100%; margin: 9px 0 0; color: #89999d; font-size: 10px; text-align: center; }
.redirect-url { width: 100%; margin-top: 7px; padding: 8px 9px; border-radius: 9px; color: #4c7277; background: #edf4f3; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 9px; line-height: 1.5; overflow-wrap: anywhere; }
.qr-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; width: 100%; margin-top: 10px; }
.qr-actions :deep(.van-button) { min-height: 44px; padding: 0 7px; color: #397773; border-color: #cfe2df; font-size: 11px; }
.click-disclaimer { margin: 8px 0 0; color: #8b999c; font-size: 9px; text-align: center; }
.qr-empty { display: flex; align-items: center; gap: 10px; margin-top: 13px; padding: 13px 11px; border: 1px dashed #cfdfdf; border-radius: 13px; color: #819296; font-size: 11px; line-height: 1.5; }
.qr-empty :deep(.van-icon) { flex: 0 0 auto; color: #35817b; font-size: 21px; }
.capability-note { padding: 13px 15px; color: #668088; background: #f0f6f7; font-size: 10px; line-height: 1.65; box-shadow: none; }
.capability-note strong { color: #30545d; font-size: 11px; }
.capability-note p { margin: 4px 0 0; }
@media (min-width: 600px) { .page-content { max-width: 580px; margin: 0 auto; } }
</style>
