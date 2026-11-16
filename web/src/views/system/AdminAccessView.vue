<template>
  <div class="access-page">
    <header class="page-heading">
      <div>
        <p class="eyebrow">SECURITY · ADMIN ACCESS</p>
        <h1>管理员账号与权限</h1>
        <p class="intro">普通管理员默认没有业务权限；只给岗位所需权限。超管角色不出现在普通角色编辑和账号列表中。</p>
      </div>
      <el-button :loading="loading" @click="loadAll">刷新</el-button>
    </header>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="按最小权限分配"
      description="只读、运营、财务和系统管理权限控制业务模块；查看完整客户联系方式与导出客户/订单数据需额外单独授权并记录审计。角色变更或禁用账号会在下一次请求立即生效；管理员账号和角色配置只允许超管操作。"
    />

    <section class="permission-grid">
      <article v-for="permission in permissions" :key="permission.code" class="permission-card">
        <span class="permission-code">{{ permission.code }}</span>
        <strong>{{ permission.name }}</strong>
        <p>{{ permission.description }}</p>
      </article>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div><h2>权限角色</h2><p>一个角色可以组合多个权限组；空权限角色无法访问业务模块。</p></div>
        <el-button type="primary" @click="openCreateRole">新建角色</el-button>
      </div>
      <el-table :data="roles" v-loading="loading" row-key="id">
        <el-table-column prop="name" label="角色名称" min-width="150" />
        <el-table-column prop="code" label="角色代码" min-width="150" />
        <el-table-column label="权限组" min-width="300">
          <template #default="{ row }">
            <div class="tag-list">
              <el-tag v-for="code in row.permissions" :key="code" size="small" effect="plain">{{ permissionName(code) }}</el-tag>
              <el-tag v-if="!row.permissions?.length" size="small" type="info" effect="plain">无权限</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" align="right">
          <template #default="{ row }"><el-button link type="primary" @click="openEditRole(row)">编辑</el-button></template>
        </el-table-column>
        <template #empty><el-empty description="尚未创建普通管理员角色" /></template>
      </el-table>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div><h2>普通管理员账号</h2><p>初始密码只在创建时提交一次，不会在列表或 API 响应中回显。</p></div>
        <el-button type="primary" :disabled="!roles.length" @click="openCreateAdmin">新建管理员</el-button>
      </div>
      <el-table :data="admins" v-loading="loading" row-key="id">
        <el-table-column prop="username" label="登录账号" min-width="145" />
        <el-table-column prop="display_name" label="显示名称" min-width="140" />
        <el-table-column label="角色 / 权限" min-width="250">
          <template #default="{ row }">
            <strong>{{ row.role_name }}</strong>
            <div class="tag-list compact-tags">
              <el-tag v-for="code in row.permissions" :key="code" size="small" effect="plain">{{ permissionName(code) }}</el-tag>
              <el-tag v-if="!row.permissions?.length" size="small" type="info" effect="plain">无权限</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '启用' : '已停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="100" align="right">
          <template #default="{ row }"><el-button link type="primary" @click="openEditAdmin(row)">编辑</el-button></template>
        </el-table-column>
        <template #empty><el-empty description="暂无普通管理员账号" /></template>
      </el-table>
    </section>

    <el-dialog v-model="roleDialog" :title="roleForm.id ? '编辑权限角色' : '新建权限角色'" width="min(560px, 94vw)" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="角色名称" required><el-input v-model="roleForm.name" maxlength="60" placeholder="例如：门店运营专员" /></el-form-item>
        <el-form-item v-if="!roleForm.id" label="角色代码" required><el-input v-model="roleForm.code" maxlength="60" placeholder="例如：store_operator" /></el-form-item>
        <el-form-item label="权限组">
          <el-checkbox-group v-model="roleForm.permissions" class="permission-options">
            <el-checkbox v-for="permission in permissions" :key="permission.code" :value="permission.code">
              <span>{{ permission.name }}</span>
              <small>{{ permission.description }}</small>
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-alert type="info" :closable="false" title="账号与角色管理仅限超管；“系统管理”权限不包含超管账号管理。" />
      </el-form>
      <template #footer><el-button @click="roleDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveRole">保存角色</el-button></template>
    </el-dialog>

    <el-dialog v-model="adminDialog" :title="adminForm.id ? '编辑管理员账号' : '新建管理员账号'" width="min(520px, 94vw)" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="登录账号" required><el-input v-model="adminForm.username" :disabled="Boolean(adminForm.id)" maxlength="60" placeholder="3–60 位字母、数字、点、下划线或短横线" /></el-form-item>
        <el-form-item label="显示名称" required><el-input v-model="adminForm.display_name" maxlength="80" placeholder="例如：北区运营" /></el-form-item>
        <el-form-item v-if="!adminForm.id" label="初始密码" required><el-input v-model="adminForm.password" type="password" show-password autocomplete="new-password" placeholder="至少 12 位" /></el-form-item>
        <el-form-item label="权限角色" required><el-select v-model="adminForm.role_id" class="full-width" placeholder="选择普通管理员角色"><el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" /></el-select></el-form-item>
        <el-form-item v-if="adminForm.id" label="账号状态"><el-radio-group v-model="adminForm.status"><el-radio value="active">启用</el-radio><el-radio value="disabled">停用</el-radio></el-radio-group></el-form-item>
        <el-alert v-if="!adminForm.id" type="info" :closable="false" title="密码会以哈希形式保存；创建后不再显示，请由管理员安全地转交给本人。" />
      </el-form>
      <template #footer><el-button @click="adminDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveAdmin">{{ adminForm.id ? '保存账号' : '创建账号' }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  createAdminAccount,
  createAdminRole,
  fetchAdminAccounts,
  fetchAdminPermissionCatalog,
  fetchAdminRoles,
  updateAdminAccount,
  updateAdminRole
} from '../../api/modules'

const permissions = ref([])
const roles = ref([])
const admins = ref([])
const loading = ref(false)
const saving = ref(false)
const roleDialog = ref(false)
const adminDialog = ref(false)
const roleForm = reactive({ id: 0, name: '', code: '', permissions: [] })
const adminForm = reactive({ id: 0, username: '', display_name: '', password: '', role_id: null, status: 'active' })
const permissionName = code => permissions.value.find(item => item.code === code)?.name || code

const loadAll = async () => {
  loading.value = true
  try {
    const [permissionResponse, roleResponse, adminResponse] = await Promise.all([
      fetchAdminPermissionCatalog(), fetchAdminRoles(), fetchAdminAccounts()
    ])
    permissions.value = permissionResponse.data?.list || []
    roles.value = roleResponse.data?.list || []
    admins.value = adminResponse.data?.list || []
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '加载管理员权限数据失败')
  } finally {
    loading.value = false
  }
}

const openCreateRole = () => {
  Object.assign(roleForm, { id: 0, name: '', code: '', permissions: [] })
  roleDialog.value = true
}
const openEditRole = role => {
  Object.assign(roleForm, { id: role.id, name: role.name, code: role.code, permissions: [...(role.permissions || [])] })
  roleDialog.value = true
}
const saveRole = async () => {
  if (!roleForm.name.trim() || (!roleForm.id && !/^[a-z][a-z0-9_-]{2,59}$/.test(roleForm.code))) {
    ElMessage.warning('请填写有效角色名称和角色代码')
    return
  }
  saving.value = true
  try {
    const payload = { name: roleForm.name.trim(), permissions: [...roleForm.permissions] }
    if (roleForm.id) await updateAdminRole(roleForm.id, payload)
    else await createAdminRole({ ...payload, code: roleForm.code.trim() })
    ElMessage.success('权限角色已保存')
    roleDialog.value = false
    await loadAll()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '保存角色失败')
  } finally {
    saving.value = false
  }
}
const openCreateAdmin = () => {
  Object.assign(adminForm, { id: 0, username: '', display_name: '', password: '', role_id: roles.value[0]?.id || null, status: 'active' })
  adminDialog.value = true
}
const openEditAdmin = admin => {
  Object.assign(adminForm, { id: admin.id, username: admin.username, display_name: admin.display_name, password: '', role_id: admin.role_id, status: admin.status })
  adminDialog.value = true
}
const saveAdmin = async () => {
  if (!adminForm.display_name.trim() || !adminForm.role_id || (!adminForm.id && (!adminForm.username.trim() || adminForm.password.length < 12))) {
    ElMessage.warning('请完整填写账号、显示名称、至少 12 位初始密码和角色')
    return
  }
  saving.value = true
  try {
    if (adminForm.id) {
      await updateAdminAccount(adminForm.id, { display_name: adminForm.display_name.trim(), role_id: adminForm.role_id, status: adminForm.status })
    } else {
      await createAdminAccount({ username: adminForm.username.trim(), display_name: adminForm.display_name.trim(), password: adminForm.password, role_id: adminForm.role_id })
    }
    adminForm.password = ''
    ElMessage.success(adminForm.id ? '管理员账号已更新' : '管理员账号已创建')
    adminDialog.value = false
    await loadAll()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '保存管理员账号失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.access-page{display:grid;gap:15px;color:#172b4d}.page-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:14px}.eyebrow{margin:0 0 5px;color:#2563eb;font-size:11px;font-weight:800;letter-spacing:.13em}.page-heading h1{margin:0;font-size:27px}.intro{margin:7px 0 0;color:#64748b;line-height:1.55}.permission-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.permission-card,.panel{background:#fff;border:1px solid #e2eaf4;border-radius:15px;box-shadow:0 5px 18px rgba(15,23,42,.035)}.permission-card{display:grid;gap:8px;padding:14px}.permission-code{font-size:10px;font-weight:800;letter-spacing:.1em;color:#2563eb;text-transform:uppercase}.permission-card strong{font-size:15px}.permission-card p{margin:0;color:#64748b;font-size:12px;line-height:1.55}.panel{padding:14px}.panel-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:10px}.panel-heading h2{margin:0;font-size:17px}.panel-heading p{margin:5px 0 0;color:#718096;font-size:12px;line-height:1.5}.tag-list{display:flex;gap:5px;flex-wrap:wrap}.compact-tags{margin-top:6px}.permission-options{display:grid;gap:10px;width:100%}.permission-options :deep(.el-checkbox){align-items:flex-start;height:auto;white-space:normal;margin-right:0}.permission-options :deep(.el-checkbox__label){display:grid;gap:3px;white-space:normal;line-height:1.35}.permission-options small{color:#64748b;font-size:11px}.full-width{width:100%}@media(max-width:1000px){.permission-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:650px){.page-heading{display:grid}.page-heading h1{font-size:23px}.permission-grid{grid-template-columns:1fr}.panel{padding:10px}.panel-heading{align-items:flex-start}.panel :deep(.el-table){font-size:12px}.panel :deep(.el-table .cell){padding-left:5px;padding-right:5px}}
</style>
