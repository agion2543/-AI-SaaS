# 本地生活商家 AI 运营 SaaS

一个面向本地生活商家的 AI 运营 SaaS 平台，覆盖平台后台、商家工作台和顾客扫码点单页。

## 技术栈

- 后端：Go 1.22 + Gin + GORM
- 前端：Vue3 + Element Plus + Vite
- 数据库：MySQL 8.0
- 支付：支付宝沙箱/页面支付基础接入
- 部署：Docker + Docker Compose + Nginx

## 当前功能

平台端：

- 运营总览、收入统计、商家排行、风险巡检
- 商家运营、订阅开通、冻结、收款审核
- 顾客数据沉淀、AI 标签、消费统计
- 平台套餐、订单财务、退款售后
- 操作审计日志

商家端：

- 商家注册、登录、订阅付费
- 经营看板、AI 经营分析
- 门店管理、商品管理、优惠活动
- 订单管理、订单详情、小票、退款记录
- 财务对账、收款设置

顾客端：

- 扫码进入门店
- 浏览商品、购物车、下单支付
- 订单成功页

## 目录结构

```text
cmd/server                 # Go 服务启动入口
internal                   # 后端核心代码
internal/controller        # 控制器
internal/service           # 业务逻辑
internal/model             # GORM 模型
internal/repository        # 数据访问
internal/router            # 路由
sql/init.sql               # 初始化 SQL
sql/migrations             # 迁移 SQL
web                        # Vue3 前端
docker                     # Nginx 配置
docs                       # 文档
```

## 本地开发

后端：

```powershell
Copy-Item .env.example .env
go mod tidy
go run ./cmd/server
```

前端：

```powershell
cd web
npm install
npm run dev
```

默认地址：

- 平台后台：http://localhost:5173/admin/dashboard
- 商家端：http://localhost:5173/merchant/login
- 顾客端：http://localhost:5173/customer/store/5
- 后端健康检查：http://localhost:8080/health

默认管理员：

```text
账号：admin
密码：Admin@123456
```

## 生产部署

生产部署说明见：

[docs/production-deploy.md](docs/production-deploy.md)

快速启动：

```bash
cp .env.production.example .env.production
docker compose -f docker-compose.prod.yml up -d --build
```

上线前请务必修改：

- `JWT_SECRET`
- `AES_SECRET`
- MySQL 密码
- 管理员默认密码
- 支付宝 APPID、公钥、私钥、回调地址

## GitHub 上传注意

项目必须保留目录结构上传。不要把所有文件平铺到仓库根目录，否则 Go 后端和 Vue 前端都无法正常构建。

正确示例：

```text
internal/router/router.go
web/src/router/index.js
sql/migrations/20260430_add_audit_logs.sql
```

错误示例：

```text
router.go
index.js
20260430_add_audit_logs.sql
```

## 后续重点

- 顾客订单状态页和支付失败重试
- 商家真实收款账户接入
- 支付宝/微信真实退款接口
- 商品分类和图片上传
- 更完整的财务导出和对账
- 生产环境 HTTPS、备份、监控和日志告警
