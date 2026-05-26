# 移动端 H5 开发说明

## 项目结构

```
web/
├── src/
│   ├── mobile/                    # 移动端页面
│   │   ├── customer/              # 顾客端（扫码点单）
│   │   │   ├── CustomerStore.vue          # 门店商品页
│   │   │   ├── CustomerCart.vue           # 购物车页
│   │   │   ├── CustomerOrderConfirm.vue   # 订单确认页
│   │   │   └── CustomerOrderStatus.vue    # 订单状态页
│   │   └── merchant/              # 商家端
│   │       ├── MerchantLayout.vue          # 商家端布局（含底部导航）
│   │       ├── MerchantLogin.vue           # 商家登录
│   │       ├── MerchantRegister.vue        # 商家注册
│   │       ├── MerchantHome.vue            # 工作台首页
│   │       ├── MerchantOrders.vue          # 订单管理
│   │       ├── MerchantOrderDetail.vue     # 订单详情
│   │       ├── MerchantProducts.vue        # 商品管理
│   │       ├── MerchantStores.vue          # 门店管理
│   │       ├── MerchantFinance.vue         # 财务管理
│   │       ├── MerchantCoupons.vue         # 优惠券管理
│   │       ├── MerchantAI.vue              # AI 运营助手
│   │       └── MerchantSettings.vue       # 设置页
│   ├── router/
│   │   └── mobile.js              # 移动端路由配置
│   ├── components/
│   │   └── mobile/                # 移动端通用组件
│   └── styles/
│       └── mobile.css            # 移动端全局样式
├── mobile.html                   # 移动端入口 HTML
└── mobile-main.js                # 移动端入口 JS
```

## 运行方式

### 开发环境

1. 启动后端服务：
```bash
cd go-web-gin-health
go run main.go
```

2. 启动前端桌面版：
```bash
cd web
npm run dev
```

3. 启动前端移动版（新建终端）：
```bash
cd web
npm run dev:mobile
# 或直接访问 http://localhost:5173/mobile.html
```

### 生产构建

```bash
cd web
npm run build
# 构建完成后，dist/ 目录包含 desktop 和 mobile 两个版本
```

## 移动端路由

### 顾客端（扫码点单）

| 路由 | 页面 | 说明 |
|------|------|------|
| `/m/store/:storeId` | 门店商品页 | 扫码入口 |
| `/m/cart/:storeId` | 购物车 | 商品结算 |
| `/m/order/confirm` | 订单确认 | 提交订单 |
| `/m/order/:orderNo` | 订单状态 | 支付/追踪 |

### 商家端

| 路由 | 页面 | 说明 |
|------|------|------|
| `/m/merchant/login` | 登录 | 手机号/密码登录 |
| `/m/merchant/register` | 注册 | 商家入驻 |
| `/m/merchant/home` | 工作台 | 经营概览 |
| `/m/merchant/orders` | 订单管理 | 订单列表/处理 |
| `/m/merchant/orders/:id` | 订单详情 | 订单详情 |
| `/m/merchant/products` | 商品管理 | 商品上下架 |
| `/m/merchant/stores` | 门店管理 | 门店信息 |
| `/m/merchant/finance` | 财务管理 | 营收统计 |
| `/m/merchant/coupons` | 优惠券 | 优惠券管理 |
| `/m/merchant/ai` | AI 助手 | AI 运营工具 |
| `/m/merchant/settings` | 设置 | 账号设置 |

## 技术栈

- **框架**: Vue 3
- **构建工具**: Vite
- **移动端组件库**: Vant 4
- **路由**: Vue Router 4
- **状态管理**: Pinia（复用）
- **HTTP**: Axios（复用现有 API）

## 开发指南

### 添加新页面

1. 在 `src/mobile/merchant/` 或 `src/mobile/customer/` 目录下创建 Vue 文件
2. 在 `src/router/mobile.js` 中添加路由配置
3. 使用 Vant 组件库构建 UI
4. 复用 `src/api/modules.js` 中的 API 方法

### 组件规范

- 使用 Vant 组件库，保持移动端一致性
- 按钮最小高度 44px，保证触控体验
- 表单输入框最小高度 44px
- 列表项可点击区域覆盖整行

### 样式规范

- 使用 `rem` 或 Vant 的样式变量
- 避免使用固定的 px 值
- 使用 `safe-area-inset-*` 处理刘海屏
- 关键交互使用微动效提升体验

## 注意事项

### 开发阶段

- 微信 H5 支付需要备案域名，暂时使用模拟支付
- 支付接口使用 `/customer/orders` 相关 API
- 订单状态通过轮询更新

### 生产环境

- 需要完成 ICP 备案
- 需要开通微信支付商户号
- 配置微信 H5 支付回调

## 微信集成

### JS-SDK（开发阶段）

```javascript
// 分享到朋友圈
import { useWechatShare } from '@/utils/wechat'
useWechatShare({
  title: '门店名称',
  desc: '欢迎光临',
  link: window.location.href,
  imgUrl: 'logo.png'
})
```

### H5 支付

```javascript
// 调用后端创建支付
const res = await axios.post('/v1/payment/h5-pay', {
  order_no: orderNo,
  amount: amount
})
// 跳转微信支付中间页
window.location.href = res.data.h5_url
```

## 后续开发计划

- [ ] 微信 JS-SDK 完整集成
- [ ] 微信 H5 支付正式接入
- [ ] 支付宝支付集成
- [ ] 小程序版本开发
- [ ] PWA 支持
- [ ] 消息推送集成
