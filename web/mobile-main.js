import { createApp } from 'vue'
import App from './src/AppMobile.vue'
import router from './src/router/mobile'

// 导入 Vant
import Vant from 'vant'
import 'vant/lib/index.css'

// 导入移动端全局样式
import './src/styles/mobile.css'
import './src/styles/main.css'

const app = createApp(App)

app.use(router)
app.use(Vant)

app.mount('#app')
