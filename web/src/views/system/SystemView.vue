<template>
  <div class="page-card">
    <h2 class="page-title">系统配置</h2>
    <el-form :model="form" label-width="120px">
      <el-form-item label="站点名称">
        <el-input v-model="form.site_name" />
      </el-form-item>
      <el-form-item label="支付接口">
        <el-input v-model="form.payment_gateway" type="textarea" :rows="4" />
      </el-form-item>
      <el-form-item label="备案信息">
        <el-input v-model="form.filing_info" />
      </el-form-item>
      <el-form-item label="公告设置">
        <el-input v-model="form.notice" type="textarea" :rows="4" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="save">保存配置</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchConfigs, saveConfigs } from '../../api/modules'

const form = reactive({
  site_name: '商业化 SaaS 平台',
  payment_gateway: '{"provider":"wechat","mch_id":"demo"}',
  filing_info: '粤ICP备12345678号',
  notice: '欢迎使用商业化 SaaS 付费管理系统。'
})

onMounted(async () => {
  const res = await fetchConfigs()
  res.data.forEach((item) => {
    if (item.config_key in form) {
      form[item.config_key] = item.config_value
    }
  })
})

const save = async () => {
  await saveConfigs(form)
  ElMessage.success('配置已保存')
}
</script>
