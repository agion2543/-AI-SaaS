import { createApp, ref } from 'vue'
import { createPinia } from 'pinia'
import {
  ElAlert,
  ElBadge,
  ElButton,
  ElCard,
  ElCheckbox,
  ElCheckboxGroup,
  ElCol,
  ElCollapse,
  ElCollapseItem,
  ElConfigProvider,
  ElDatePicker,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElDrawer,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElLink,
  ElLoading,
  ElMenu,
  ElMenuItem,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElProgress,
  ElRadio,
  ElRadioButton,
  ElRadioGroup,
  ElRate,
  ElRow,
  ElSegmented,
  ElSelect,
  ElSkeleton,
  ElSubMenu,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElText,
  ElTimeline,
  ElTimelineItem,
  ElUpload,
  localeContextKey,
} from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/es/components/base/style/css'
import 'element-plus/es/components/alert/style/css'
import 'element-plus/es/components/badge/style/css'
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/card/style/css'
import 'element-plus/es/components/checkbox/style/css'
import 'element-plus/es/components/checkbox-group/style/css'
import 'element-plus/es/components/col/style/css'
import 'element-plus/es/components/collapse/style/css'
import 'element-plus/es/components/collapse-item/style/css'
import 'element-plus/es/components/config-provider/style/css'
import 'element-plus/es/components/date-picker/style/css'
import 'element-plus/es/components/descriptions/style/css'
import 'element-plus/es/components/descriptions-item/style/css'
import 'element-plus/es/components/dialog/style/css'
import 'element-plus/es/components/drawer/style/css'
import 'element-plus/es/components/empty/style/css'
import 'element-plus/es/components/form/style/css'
import 'element-plus/es/components/form-item/style/css'
import 'element-plus/es/components/input/style/css'
import 'element-plus/es/components/input-number/style/css'
import 'element-plus/es/components/link/style/css'
import 'element-plus/es/components/loading/style/css'
import 'element-plus/es/components/menu/style/css'
import 'element-plus/es/components/menu-item/style/css'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import 'element-plus/es/components/option/style/css'
import 'element-plus/es/components/pagination/style/css'
import 'element-plus/es/components/progress/style/css'
import 'element-plus/es/components/radio/style/css'
import 'element-plus/es/components/radio-button/style/css'
import 'element-plus/es/components/radio-group/style/css'
import 'element-plus/es/components/rate/style/css'
import 'element-plus/es/components/row/style/css'
import 'element-plus/es/components/segmented/style/css'
import 'element-plus/es/components/select/style/css'
import 'element-plus/es/components/skeleton/style/css'
import 'element-plus/es/components/sub-menu/style/css'
import 'element-plus/es/components/switch/style/css'
import 'element-plus/es/components/tab-pane/style/css'
import 'element-plus/es/components/table/style/css'
import 'element-plus/es/components/table-column/style/css'
import 'element-plus/es/components/tabs/style/css'
import 'element-plus/es/components/tag/style/css'
import 'element-plus/es/components/text/style/css'
import 'element-plus/es/components/timeline/style/css'
import 'element-plus/es/components/timeline-item/style/css'
import 'element-plus/es/components/upload/style/css'
import './styles/main.css'
import App from './App.vue'
import router from './router'

// Keep this list aligned with Element Plus tags used by the PC views.
const elementComponents = [
  ElAlert,
  ElBadge,
  ElButton,
  ElCard,
  ElCheckbox,
  ElCheckboxGroup,
  ElCol,
  ElCollapse,
  ElCollapseItem,
  ElConfigProvider,
  ElDatePicker,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElDrawer,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElLink,
  ElMenu,
  ElMenuItem,
  ElOption,
  ElPagination,
  ElProgress,
  ElRadio,
  ElRadioButton,
  ElRadioGroup,
  ElRate,
  ElRow,
  ElSegmented,
  ElSelect,
  ElSkeleton,
  ElSubMenu,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElText,
  ElTimeline,
  ElTimelineItem,
  ElUpload,
]

const app = createApp(App)
app.use(createPinia())
app.use(router)
for (const component of elementComponents) {
  app.component(component.name, component)
}
// These plugins preserve v-loading and the application context for service APIs.
app.use(ElLoading)
app.use(ElMessage)
app.use(ElMessageBox)
// Service UIs render outside the root component tree, so provide their locale on app context too.
app.provide(localeContextKey, ref(zhCn))
app.mount('#app')
