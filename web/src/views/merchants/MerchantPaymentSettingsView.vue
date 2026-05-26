<template>
  <div class="payment-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">PAYMENT SETTINGS</div>
        <h2 class="page-title">收款设置</h2>
        <p class="muted">配置商家自己的收款账户。当前阶段用于订单对账、退款追踪和后续直连支付通道切换。</p>
      </div>
      <el-tag size="large" :type="auditType(config.audit_status)">{{ auditLabel(config.audit_status) }}</el-tag>
    </section>

    <section class="page-card tips-grid">
      <div>
        <strong>推荐模式</strong>
        <span>前期建议使用“商家收款码模式”，平台只记录订单和退款流水，不代收、不清分资金。</span>
      </div>
      <div>
        <strong>安全说明</strong>
        <span>这里暂不保存私钥、证书等敏感信息。真实接入时需要加密存储并限制查看权限。</span>
      </div>
      <div>
        <strong>审核流程</strong>
        <span>商家每次修改后会回到待审核，平台审核通过后用于对账和运营巡检，不代表平台已经代收代付。</span>
      </div>
    </section>

    <section class="page-card readiness-card" :class="customerPayStatus.tone">
      <div class="readiness-head">
        <div>
          <span>PAYMENT READINESS</span>
          <strong>{{ customerPayStatus.title }}</strong>
          <p>{{ customerPayStatus.description }}</p>
        </div>
        <el-tag size="large" :type="customerPayStatus.tag">{{ customerPayStatus.badge }}</el-tag>
      </div>
      <div class="readiness-steps">
        <div v-for="item in readinessSteps" :key="item.label" :class="{ done: item.done, active: item.active }">
          <span>{{ item.done ? '已完成' : item.active ? '处理中' : '待处理' }}</span>
          <strong>{{ item.label }}</strong>
          <small>{{ item.hint }}</small>
        </div>
      </div>
    </section>

    <section class="page-card form-card">
      <el-form label-width="120px" :model="form">
        <el-form-item label="收款渠道">
          <el-radio-group v-model="form.channel">
            <el-radio-button label="alipay">支付宝</el-radio-button>
            <el-radio-button label="wechat">微信支付</el-radio-button>
            <el-radio-button label="bank">银行卡</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="收款模式">
          <el-select v-model="form.mode" placeholder="请选择收款模式">
            <el-option label="商家收款码模式（MVP 推荐）" value="direct" />
            <el-option label="平台统一收款（后续/需合规确认）" value="platform" />
            <el-option label="服务商分账模式（后续/需资质）" value="service_provider" />
          </el-select>
        </el-form-item>
        <el-alert
          class="mode-alert"
          :type="modeAdvice.type"
          show-icon
          :closable="false"
          :title="modeAdvice.title"
          :description="modeAdvice.description"
        />
        <el-form-item label="账户名称">
          <el-input v-model="form.account_name" placeholder="例如：炭火小院烧烤 / 公司主体名称" />
        </el-form-item>
        <el-form-item label="收款账号">
          <el-input v-model="form.account_no" placeholder="支付宝账号、微信商户号或银行卡号" />
        </el-form-item>
        <el-alert
          v-if="accountWarning"
          class="qr-alert"
          type="warning"
          show-icon
          :closable="false"
          :title="accountWarning"
        />
        <el-form-item label="支付宝收款码">
          <div class="qr-input-line">
            <el-input v-model="form.alipay_qr_code" placeholder="上传收款码图片，或填写支付宝收款链接" />
            <el-upload :show-file-list="false" :http-request="(options) => uploadQr(options, 'alipay')" accept="image/*">
              <el-button :loading="uploadingKey === 'alipay'">上传图片</el-button>
            </el-upload>
          </div>
          <small class="field-tip">推荐直接上传支付宝收款码图片，顾客订单页会展示为付款入口。</small>
        </el-form-item>
        <el-form-item label="微信收款码">
          <div class="qr-input-line">
            <el-input v-model="form.wechat_qr_code" placeholder="上传收款码图片，或填写微信收款链接" />
            <el-upload :show-file-list="false" :http-request="(options) => uploadQr(options, 'wechat')" accept="image/*">
              <el-button :loading="uploadingKey === 'wechat'">上传图片</el-button>
            </el-upload>
          </div>
          <small class="field-tip">推荐直接上传微信收款码图片，减少商家手动复制链接的麻烦。</small>
        </el-form-item>
        <el-alert
          v-if="qrWarning"
          class="qr-alert"
          type="warning"
          show-icon
          :closable="false"
          :title="qrWarning"
        />
        <div class="qr-preview-grid">
          <article v-for="item in qrPreviewItems" :key="item.key" class="qr-preview-card" :class="{ empty: !item.value }">
            <div class="qr-preview-head">
              <strong>{{ item.label }}</strong>
              <el-tag size="small" :type="item.value ? 'success' : 'info'">{{ item.value ? '已配置' : '未配置' }}</el-tag>
            </div>
            <div class="qr-preview-box">
              <img v-if="item.value" :src="qrDisplayUrl(item.value, item.key)" :alt="item.label" @error="markQrBroken(item.key)" />
              <span v-else>暂无收款码</span>
              <span v-if="brokenQrs[item.key]" class="qr-error">图片加载失败，将按文本生成二维码展示给顾客。</span>
            </div>
            <p>{{ item.tip }}</p>
          </article>
        </div>
        <el-form-item label="应用 APPID">
          <el-input v-model="form.app_id" placeholder="支付宝/微信应用 APPID，没有可暂不填" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="form.contact_phone" maxlength="11" placeholder="用于平台审核联系" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" maxlength="255" show-word-limit placeholder="例如：测试环境、门店收款账户、财务联系人等" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">{{ saveButtonText }}</el-button>
          <el-button :loading="loading" @click="load">重新加载</el-button>
        </el-form-item>
      </el-form>
    </section>

    <section class="page-card status-card">
      <h3>当前审核状态</h3>
      <div class="status-grid">
        <div><span>启用状态</span><strong>{{ statusLabel(config.status) }}</strong></div>
        <div><span>审核状态</span><strong>{{ auditLabel(config.audit_status) }}</strong></div>
        <div><span>审核备注</span><strong>{{ config.audit_remark || '-' }}</strong></div>
        <div><span>更新时间</span><strong>{{ formatTime(config.updated_at) }}</strong></div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchMerchantPaymentConfig, saveMerchantPaymentConfig, uploadMerchantProductImage } from '../../api/modules'

const loading = ref(false)
const saving = ref(false)
const config = ref({})
const brokenQrs = ref({})
const uploadingKey = ref('')
const form = reactive({
  channel: 'alipay',
  mode: 'direct',
  account_name: '',
  account_no: '',
  alipay_qr_code: '',
  wechat_qr_code: '',
  app_id: '',
  contact_phone: '',
  remark: ''
})

const modeAdviceMap = {
  direct: {
    type: 'success',
    title: 'MVP 推荐：商家收款码模式',
    description: '顾客点单款进入商家自己的支付宝/微信/银行卡账户，平台负责记录订单、退款和对账数据，低成本且合规边界更清楚。'
  },
  platform: {
    type: 'warning',
    title: '后续模式：平台统一收款',
    description: '平台统一收款后再结算给商家，可能涉及资金清分、二清和对账合规风险，建议完成支付资质、合同和风控流程后再启用。'
  },
  service_provider: {
    type: 'warning',
    title: '后续模式：服务商分账',
    description: '适合规模化后接入支付宝/微信官方服务商或分账能力，需要主体资质、商户进件、回调和结算对账能力配套。'
  }
}

const modeAdvice = computed(() => modeAdviceMap[form.mode] || modeAdviceMap.direct)
const qrPreviewItems = computed(() => [
  { key: 'alipay', label: '支付宝收款码', value: form.alipay_qr_code.trim(), tip: '顾客订单页会展示为支付宝付款入口。' },
  { key: 'wechat', label: '微信收款码', value: form.wechat_qr_code.trim(), tip: '顾客订单页会展示为微信付款入口。' }
])
const hasAnyQrCode = computed(() => qrPreviewItems.value.some((item) => Boolean(item.value)))
const hasBasicAccount = computed(() => Boolean(form.account_name.trim() && form.account_no.trim()))
const isApprovedAndEnabled = computed(() => config.value.audit_status === 'approved' && config.value.status === 'enabled')
const accountWarning = computed(() => {
  if (!form.account_name.trim() && !form.account_no.trim()) return '请填写账户名称和收款账号，方便平台审核和商家后续对账。'
  if (!form.account_name.trim()) return '请填写账户名称，例如门店名称、个人收款主体或公司主体名称。'
  if (!form.account_no.trim()) return '请填写收款账号，例如支付宝账号、微信号、商户号或银行卡号。'
  return ''
})
const qrWarning = computed(() => {
  if (form.mode !== 'direct') return ''
  if (!hasAnyQrCode.value) return '商家收款码模式至少需要填写一个收款码，否则顾客无法在订单页扫码付款。'
  if (qrPreviewItems.value.some((item) => item.value.length > 1000)) return '收款码链接过长，请控制在 1000 字以内。'
  return ''
})
const customerPayStatus = computed(() => {
  if (isApprovedAndEnabled.value) {
    return {
      tone: 'success',
      tag: 'success',
      badge: '顾客端可见',
      title: '收款码已启用，顾客订单页会展示付款入口',
      description: '顾客付款会直接进入你的收款账户。顾客点击“我已付款”后，请在商家订单页核对到账并确认收款。'
    }
  }
  if (config.value.audit_status === 'rejected') {
    return {
      tone: 'danger',
      tag: 'danger',
      badge: '需修改',
      title: '收款资料被驳回，顾客端暂不展示收款码',
      description: config.value.audit_remark || '请按平台审核备注修改账户信息或收款码后重新提交审核。'
    }
  }
  if (config.value.id && config.value.audit_status === 'pending') {
    return {
      tone: 'warning',
      tag: 'warning',
      badge: '待平台审核',
      title: '已提交收款资料，等待平台审核启用',
      description: '审核通过前，顾客端不会展示收款码；如有订单，可先按门店现场方式收款。'
    }
  }
  if (!hasAnyQrCode.value) {
    return {
      tone: 'warning',
      tag: 'warning',
      badge: '缺少收款码',
      title: '请先上传支付宝或微信收款码',
      description: '商家收款码模式至少需要一个收款码，平台审核通过后才会展示给顾客付款。'
    }
  }
  return {
    tone: 'info',
    tag: 'info',
    badge: '未提交',
    title: '收款资料尚未提交审核',
    description: '请确认账户名称、账号和收款码无误后提交审核。'
  }
})
const readinessSteps = computed(() => [
  {
    label: '填写收款账户',
    done: hasBasicAccount.value,
    active: !hasBasicAccount.value,
    hint: hasBasicAccount.value ? '账户名称和账号已填写' : '请填写收款账户名称和账号'
  },
  {
    label: '上传收款码',
    done: hasAnyQrCode.value,
    active: hasBasicAccount.value && !hasAnyQrCode.value,
    hint: hasAnyQrCode.value ? qrPreviewItems.value.filter((item) => item.value).map((item) => item.label.replace('收款码', '')).join('、') : '支付宝或微信至少上传一个'
  },
  {
    label: '平台审核启用',
    done: isApprovedAndEnabled.value,
    active: config.value.audit_status === 'pending',
    hint: isApprovedAndEnabled.value ? '顾客端会展示收款码' : config.value.audit_status === 'rejected' ? '请修改后重新提交' : '提交后等待平台审核'
  }
])
const saveButtonText = computed(() => {
  if (config.value.audit_status === 'rejected') return '修改后重新提交审核'
  if (config.value.audit_status === 'approved') return '保存修改并重新审核'
  return '保存并提交审核'
})

const load = async () => {
  loading.value = true
  try {
    const res = await fetchMerchantPaymentConfig()
    config.value = res.data.config || {}
    Object.assign(form, {
      channel: config.value.channel || 'alipay',
      mode: config.value.mode || 'direct',
      account_name: config.value.account_name || '',
      account_no: config.value.account_no || '',
      alipay_qr_code: config.value.alipay_qr_code || '',
      wechat_qr_code: config.value.wechat_qr_code || '',
      app_id: config.value.app_id || '',
      contact_phone: config.value.contact_phone || '',
      remark: config.value.remark || ''
    })
  } finally {
    loading.value = false
  }
}

const save = async () => {
  if (accountWarning.value) {
    ElMessage.warning(accountWarning.value)
    return
  }
  if (qrWarning.value) {
    ElMessage.warning(qrWarning.value)
    return
  }
  saving.value = true
  try {
    const res = await saveMerchantPaymentConfig({ ...form })
    config.value = res.data.config || {}
    ElMessage.success('收款信息已提交，等待平台审核')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '收款信息提交失败，请检查后重试')
  } finally {
    saving.value = false
  }
}

const uploadQr = async (options, key) => {
  const file = options.file
  if (!file?.type?.startsWith('image/')) {
    ElMessage.warning('请上传图片格式的收款码')
    options.onError?.(new Error('invalid image type'))
    return
  }
  if (file.size > 3 * 1024 * 1024) {
    ElMessage.warning('收款码图片不能超过 3MB')
    options.onError?.(new Error('image too large'))
    return
  }
  uploadingKey.value = key
  try {
    const formData = new FormData()
    formData.append('file', file)
    const res = await uploadMerchantProductImage(formData)
    if (key === 'alipay') form.alipay_qr_code = res.data.url || ''
    if (key === 'wechat') form.wechat_qr_code = res.data.url || ''
    ElMessage.success('收款码已上传')
    options.onSuccess?.(res.data)
  } catch (error) {
    options.onError?.(error)
    ElMessage.error(error?.response?.data?.message || '收款码上传失败')
  } finally {
    uploadingKey.value = ''
  }
}

const auditLabel = (value) => ({ pending: '待审核', approved: '审核通过', rejected: '审核驳回' }[value] || '待填写')
const auditType = (value) => ({ pending: 'warning', approved: 'success', rejected: 'danger' }[value] || 'info')
const statusLabel = (value) => ({ enabled: '已启用', disabled: '未启用' }[value] || '未启用')
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'
const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const qrDisplayUrl = (value, key = '') => {
  const text = String(value || '').trim()
  if (!text) return ''
  if (key && brokenQrs.value[key]) return buildQrImage(text)
  if (text.startsWith('data:image/')) return text
  if (/\.(png|jpe?g|webp|gif|svg)(\?.*)?$/i.test(text)) return text
  return buildQrImage(text)
}
const markQrBroken = (key) => {
  brokenQrs.value = { ...brokenQrs.value, [key]: true }
}

onMounted(load)

watch(() => [form.alipay_qr_code, form.wechat_qr_code], () => {
  brokenQrs.value = {}
})
</script>

<style scoped>
.payment-stack {
  display: grid;
  gap: 18px;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: flex-start;
  padding: 22px;
}

.eyebrow {
  color: #0ea5e9;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.muted {
  color: #64748b;
  margin: 0;
  line-height: 1.7;
}

.tips-grid,
.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
}

.tips-grid div,
.status-grid div {
  padding: 16px;
  border-radius: 16px;
  background: #f8fbff;
  border: 1px solid #e5edf9;
}

.tips-grid strong,
.tips-grid span,
.status-grid span,
.status-grid strong {
  display: block;
}

.tips-grid span,
.status-grid span {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.6;
}

.readiness-card {
  display: grid;
  gap: 16px;
  padding: 18px;
  border: 1px solid #dbeafe;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.readiness-card.success {
  border-color: #bbf7d0;
  background: linear-gradient(135deg, #ecfdf5, #ffffff);
}

.readiness-card.warning {
  border-color: #fed7aa;
  background: linear-gradient(135deg, #fff7ed, #ffffff);
}

.readiness-card.danger {
  border-color: #fecaca;
  background: linear-gradient(135deg, #fef2f2, #ffffff);
}

.readiness-head {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: flex-start;
}

.readiness-head span,
.readiness-head strong,
.readiness-head p {
  display: block;
}

.readiness-head span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.12em;
}

.readiness-head strong {
  margin-top: 5px;
  color: #0f2747;
  font-size: 20px;
}

.readiness-head p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
}

.readiness-steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.readiness-steps div {
  padding: 13px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.8);
}

.readiness-steps div.done {
  border-color: #bbf7d0;
  background: #ecfdf5;
}

.readiness-steps div.active {
  border-color: #93c5fd;
  background: #eff6ff;
}

.readiness-steps span,
.readiness-steps strong,
.readiness-steps small {
  display: block;
}

.readiness-steps span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.readiness-steps strong {
  margin-top: 6px;
  color: #0f2747;
}

.readiness-steps small {
  margin-top: 5px;
  color: #64748b;
  line-height: 1.45;
}

.form-card,
.status-card {
  padding: 22px;
}

.mode-alert {
  margin: -4px 0 18px;
}

.field-tip {
  display: block;
  margin-top: 6px;
  color: #64748b;
  line-height: 1.5;
}

.qr-input-line {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  width: 100%;
}

.qr-alert {
  margin: -4px 0 18px;
}

.qr-preview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
  margin: 0 0 18px 120px;
}

.qr-preview-card {
  padding: 14px;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.qr-preview-card.empty {
  border-color: #e5e7eb;
  background: #f8fafc;
}

.qr-preview-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
}

.qr-preview-box {
  min-height: 154px;
  display: grid;
  place-items: center;
  padding: 12px;
  border: 1px dashed #bfdbfe;
  border-radius: 14px;
  background: #ffffff;
  color: #94a3b8;
}

.qr-preview-box img {
  width: 132px;
  height: 132px;
  object-fit: contain;
}

.qr-error {
  margin-top: 8px;
  color: #b45309;
  font-size: 12px;
  text-align: center;
}

.qr-preview-card p {
  margin: 10px 0 0;
  color: #64748b;
  line-height: 1.6;
}

@media (max-width: 760px) {
  .readiness-head {
    flex-direction: column;
  }

  .readiness-steps {
    grid-template-columns: 1fr;
  }

  .qr-input-line {
    grid-template-columns: 1fr;
  }

  .qr-preview-grid {
    margin-left: 0;
  }
}
</style>
