<template>
  <main class="merchant-tickets">
    <van-nav-bar title="我的工单" left-arrow fixed placeholder @click-left="router.back()" />

    <section class="tickets-section" aria-label="提交新工单">
      <van-cell-group inset>
        <van-field
          v-model="form.title"
          label="标题"
          placeholder="例如：顾客已付款但订单仍显示待确认"
          maxlength="120"
          show-word-limit
        />
        <van-field
          v-model="form.content"
          label="内容"
          type="textarea"
          rows="4"
          autosize
          placeholder="请写清楚发生了什么、涉及哪个订单或门店"
          maxlength="1000"
          show-word-limit
        />
        <van-cell title="分类">
          <template #value>
            <van-radio-group v-model="form.category" direction="horizontal" class="category-radios">
              <van-radio
                v-for="category in supportTicketCategories"
                :key="category"
                :name="category"
              >{{ supportTicketCategoryLabel(category) }}</van-radio>
            </van-radio-group>
          </template>
        </van-cell>
      </van-cell-group>
      <van-button type="primary" block :loading="submitting" :disabled="!isSupportTicketFormValid(form)" @click="submitTicket">提交工单</van-button>
      <p class="tickets-hint">提交后请在下方列表查看处理进度与超管回复；如问题未解决，请再开一单并在标题注明“跟进 #N 单”。</p>
    </section>

    <section class="tickets-section" aria-label="我的工单">
      <van-list
        v-model:loading="loading"
        :finished="finished"
        finished-text="没有更多了"
        @load="loadList"
      >
        <van-empty v-if="rows.length === 0 && finished" description="还没有提交过工单" />
        <div v-for="row in rows" :key="row.id" class="ticket-item">
          <van-cell-group inset>
            <van-cell is-link @click="openDetail(row)">
              <template #title>
                <div class="ticket-header">
                  <strong>#{{ row.id }} {{ row.title }}</strong>
                  <van-tag :type="supportTicketStatusType(row.status)">{{ supportTicketStatusLabel(row.status) }}</van-tag>
                </div>
                <div class="ticket-meta">{{ supportTicketCategoryLabel(row.category) }} · {{ formatTime(row.created_at) }}</div>
                <div v-if="row.reply" class="ticket-reply">超管回复：{{ row.reply }}</div>
              </template>
            </van-cell>
          </van-cell-group>
        </div>
      </van-list>
    </section>

    <van-popup v-model:show="detailVisible" position="bottom" round :style="{ height: '80%' }">
      <div v-if="detail" class="ticket-detail">
        <h2>#{{ detail.id }} {{ detail.title }}</h2>
        <van-cell-group inset>
          <van-cell title="分类" :value="supportTicketCategoryLabel(detail.category)" />
          <van-cell title="状态" :value="supportTicketStatusLabel(detail.status)" />
          <van-cell title="提交时间" :value="formatTime(detail.created_at)" />
          <van-cell title="内容" :label="detail.content" />
          <van-cell title="超管回复" :label="detail.reply || '暂无回复，请稍后再来查看'" />
        </van-cell-group>
        <p class="tickets-hint">如问题未解决，请再开一单并在标题注明“跟进 #{{ detail.id }} 单”。</p>
        <van-button block @click="detailVisible = false">关闭</van-button>
      </div>
    </van-popup>
  </main>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showSuccessToast, showFailToast } from 'vant'
import { createMerchantSupportTicket, fetchMerchantSupportTicket, fetchMerchantSupportTickets } from '@/api/modules'
import {
  isSupportTicketFormValid,
  supportTicketCategories,
  supportTicketCategoryLabel,
  supportTicketStatusLabel,
  supportTicketStatusType
} from '@/utils/supportTickets'

const router = useRouter()
const rows = ref([])
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const finished = ref(false)
const submitting = ref(false)
const detail = ref(null)
const detailVisible = ref(false)
const form = reactive({ title: '', content: '', category: '' })

const loadList = async () => {
  if (finished.value) return
  loading.value = true
  try {
    const { data } = await fetchMerchantSupportTickets({ page: page.value, page_size: pageSize })
    const list = Array.isArray(data?.list) ? data.list : []
    rows.value = page.value === 1 ? list : [...rows.value, ...list]
    if (rows.value.length >= Number(data?.total || 0) || list.length < pageSize) {
      finished.value = true
    } else {
      page.value += 1
    }
  } catch {
    showFailToast('加载工单列表失败')
    finished.value = true
  } finally {
    loading.value = false
  }
}

const submitTicket = async () => {
  if (submitting.value || !isSupportTicketFormValid(form)) return
  submitting.value = true
  try {
    const { data } = await createMerchantSupportTicket({
      title: form.title.trim(),
      content: form.content.trim(),
      category: form.category
    })
    showSuccessToast(`工单 #${data?.ticket?.id || ''} 已提交`)
    form.title = ''
    form.content = ''
    form.category = ''
    page.value = 1
    finished.value = false
    rows.value = []
    await loadList()
  } catch (error) {
    showFailToast(error?.response?.data?.message || '提交工单失败')
  } finally {
    submitting.value = false
  }
}

const openDetail = async (row) => {
  if (!row?.id) return
  try {
    const { data } = await fetchMerchantSupportTicket(row.id)
    detail.value = data?.ticket || row
  } catch {
    detail.value = row
  }
  detailVisible.value = true
}

const formatTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '—'
</script>

<style scoped>
.merchant-tickets {
  min-height: 100vh;
  padding: 12px 0 76px;
  background: #f7f8fa;
}

.tickets-section {
  margin-top: 12px;
  padding: 0 12px;
  display: grid;
  gap: 12px;
}

.tickets-hint {
  margin: 0;
  color: #969799;
  font-size: 12px;
  line-height: 1.6;
}

.category-radios {
  flex-wrap: wrap;
  gap: 8px 12px;
  justify-content: flex-end;
}

.ticket-item {
  margin-bottom: 10px;
}

.ticket-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ticket-header strong {
  flex: 1;
  font-size: 14px;
}

.ticket-meta {
  margin-top: 6px;
  color: #969799;
  font-size: 12px;
}

.ticket-reply {
  margin-top: 6px;
  color: #323233;
  font-size: 13px;
  line-height: 1.6;
}

.ticket-detail {
  padding: 16px;
  display: grid;
  gap: 12px;
}

.ticket-detail h2 {
  margin: 0;
  font-size: 17px;
  line-height: 1.5;
}
</style>
