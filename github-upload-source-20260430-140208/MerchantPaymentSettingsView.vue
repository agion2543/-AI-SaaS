<template>
  <div class="payment-stack">
    <section class="page-card hero-card">
      <div>
        <div class="eyebrow">PAYMENT SETTINGS</div>
        <h2 class="page-title">收款设置</h2>
        <p class="muted">配置商家自己的收款账户。当前阶段用于平台审核、财务对账和后续直连支付通道切换。</p>
      </div>
      <el-tag size="large" :type="auditType(config.audit_status)">{{ auditLabel(config.audit_status) }}</el-tag>
    </section>

    <section class="page-card tips-grid">
      <div>
        <strong>推荐模式</strong>
        <span>前期建议使用“顾客直付商家账户”，平台只记录订单和退款流水。</span>
      </div>
      <div>
        <strong>安全说明</strong>
        <span>这里暂不保存私钥、证书等敏感信息。真实接入时需要加密存储并限制查看权限。</span>
      </div>
      <div>
        <strong>审核流程</strong>
        <span>商家每次修改后会回到待审核，平台审核通过后才标记为可用配置。</span>
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
            <el-option label="顾客直付商家账户" value="direct" />
            <el-option label="平台统一收款" value="platform" />
            <el-option label="服务商分账模式" value="service_provider" />
          </el-select>
        </el-form-item>
        <el-form-item label="账户名称">
          <el-input v-model="form.account_name" placeholder="例如：炭火小院烧烤 / 公司主体名称" />
        </el-form-item>
        <el-form-item label="收款账号">
          <el-input v-model="form.account_no" placeholder="支付宝账号、微信商户号或银行卡号" />
        </el-form-item>
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
          <el-button type="primary" :loading="saving" @click="save">保存并提交审核</el-button>
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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchMerchantPaymentConfig, saveMerchantPaymentConfig } from '../../api/modules'

const loading = ref(false)
const saving = ref(false)
const config = ref({})
const form = reactive({
  channel: 'alipay',
  mode: 'direct',
  account_name: '',
  account_no: '',
  app_id: '',
  contact_phone: '',
  remark: ''
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
      app_id: config.value.app_id || '',
      contact_phone: config.value.contact_phone || '',
      remark: config.value.remark || ''
    })
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const res = await saveMerchantPaymentConfig({ ...form })
    config.value = res.data.config || {}
    ElMessage.success('收款信息已提交，等待平台审核')
  } finally {
    saving.value = false
  }
}

const auditLabel = (value) => ({ pending: '待审核', approved: '审核通过', rejected: '审核驳回' }[value] || '待填写')
const auditType = (value) => ({ pending: 'warning', approved: 'success', rejected: 'danger' }[value] || 'info')
const statusLabel = (value) => ({ enabled: '已启用', disabled: '未启用' }[value] || '未启用')
const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '-'

onMounted(load)
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

.form-card,
.status-card {
  padding: 22px;
}
</style>
