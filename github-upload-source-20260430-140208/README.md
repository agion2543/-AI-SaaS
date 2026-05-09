# 商业化 SaaS 付费管理系统

基于 `Golang + Gin + MySQL 8.0 + Vue3 + Element Plus` 的商业化 SaaS 付费管理系统，覆盖用户会员购买、订单支付、卡密兑换、后台运营和系统配置等核心场景。

## 项目目录结构

```text
go-web-gin-health
├─ cmd/server                # Gin 服务启动入口
├─ internal
│  ├─ bootstrap              # 应用初始化
│  ├─ config                 # 环境配置
│  ├─ controller             # 控制器
│  ├─ dto                    # 请求/响应 DTO
│  ├─ middleware             # 鉴权、限流、跨域、日志
│  ├─ model                  # GORM 模型
│  ├─ repository             # 数据访问层
│  ├─ router                 # RESTful 路由
│  ├─ service                # 业务服务层
│  └─ utils                  # JWT、密码、加密、统一响应
├─ sql/init.sql              # MySQL 初始化脚本
├─ docker                    # Nginx 配置
├─ docs/api-overview.md      # API 摘要
├─ web                       # Vue3 Element Plus 管理后台
├─ Dockerfile
├─ docker-compose.yml
└─ .env.example
```

## 已实现核心能力

### 用户端

- 注册、登录、找回密码
- 会员套餐读取：日卡、周卡、月卡、年卡、终身套餐
- 在线下单、自动续费标记、订单管理
- 支持待支付订单、支付回调确认、订单取消
- 支持支付宝沙箱支付：页面支付或二维码支付、验签回调、幂等开通会员
- 卡密充值兑换、权益激活
- 个人中心：到期时间、剩余额度、设备绑定、使用记录

### 管理后台

- 管理员登录、权限分组模型
- 用户管理：封禁、解封、手动开通会员
- 套餐管理：新增、编辑、上下架、价格修改
- 订单管理、支付记录、财务总览
- 卡密批量生成、列表管理
- 系统配置：站点名称、支付接口、备案信息、公告设置

### 技术方案

- RESTful API，统一返回格式
- 全局中间件：鉴权、限流、跨域、请求日志
- 密码使用 `bcrypt` 加密存储
- 支付接口等敏感配置支持 AES 加密存储
- 使用 GORM 参数化查询，避免手写拼接 SQL
- 提供 SQL、Docker、环境配置与部署说明

## 本地开发

### 1. 准备环境

- Go 1.22+
- Node.js 20+
- MySQL 8.0

### 2. 初始化后端

```powershell
Copy-Item .env.example .env
go mod tidy
go run ./cmd/server
```

服务默认运行在 [http://localhost:8080](http://localhost:8080)，健康检查地址：
[http://localhost:8080/health](http://localhost:8080/health)

### 3. 初始化前端

```powershell
cd web
npm install
npm run dev
```

管理后台默认地址：
[http://localhost:5173](http://localhost:5173)

## Docker 部署

```powershell
Copy-Item .env.example .env
docker compose up -d --build
```

## 默认账号

- 管理员账号：`admin`
- 管理员密码：`Admin@123456`

> `sql/init.sql` 已内置默认套餐、角色与管理员账号数据。

## 说明

- 当前支付流程已拆分为“创建待支付订单 -> 支付回调确认 -> 会员生效”的标准状态流，默认提供模拟回调接口，便于后续对接微信、支付宝、Stripe 等真实通道。
- 如果你要继续扩展，我建议下一步补上真实支付回调、RBAC 细粒度权限点、导出报表和用户端前台页面。

## 支付宝沙箱联调

### 环境变量

在 `.env` 中配置：

```env
ALIPAY_SANDBOX=true
ALIPAY_APP_ID=你的沙箱APPID
ALIPAY_APP_PRIVATE_KEY=你的应用私钥PEM，换行请写成 \n
ALIPAY_PUBLIC_KEY=沙箱支付宝公钥PEM，换行请写成 \n
ALIPAY_NOTIFY_URL=http://你的可访问域名/api/v1/payments/callback/alipay
ALIPAY_RETURN_URL=http://你的前端地址/payment/return
```

### 下单返回

- `pay_mode=page`：返回 `payment.payment_url` 和 `payment.payment_form`
- `pay_mode=qr`：返回 `payment.qr_code`

### 沙箱测试步骤

1. 在支付宝开放平台进入沙箱，获取 `APPID`、应用私钥、支付宝公钥和沙箱买家账号。
2. 启动后端，确保 `ALIPAY_NOTIFY_URL` 是支付宝可以访问到的地址；本地联调建议配合内网穿透。
3. 调用 `POST /api/v1/user/orders`，`payment_channel` 传 `alipay`，`pay_mode` 传 `page` 或 `qr`。
4. `page` 模式下，浏览器跳转到返回的 `payment_url`；`qr` 模式下，用二维码工具渲染 `payment.qr_code`。
5. 使用支付宝沙箱买家账号完成付款。
6. 支付宝异步通知会调用 `/api/v1/payments/callback/alipay`，服务端验签成功后将订单改为 `paid` 并开通会员。
7. 调用 `GET /api/v1/user/orders/:orderNo` 或 `GET /api/v1/user/profile` 验证会员已生效。
