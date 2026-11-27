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
- 后端存活检查：http://localhost:8080/health（仅确认 HTTP 进程响应）
- 数据库就绪检查：http://localhost:8080/ready（检查活动数据库连接）

首次创建管理员时，须在本地 `.env` 中显式设置至少 12 位的 `INITIAL_ADMIN_PASSWORD`；未设置时，服务会拒绝完成新库初始化。已有数据库中已存在管理员账号时，可以不设置该变量，启动不会更改该账号的密码、角色或状态。初始用户名默认是 `admin`，也可通过 `INITIAL_ADMIN_USERNAME` 配置。

## 本地离线回归

运行 `bash scripts/test-local-regression.sh` 执行本地后端、前端和合成测试；默认不会连接或启动数据库。覆盖范围及可选 age/MySQL 恢复演练的安全条件见[本地回归说明](docs/local-regression.md)。

## 生产部署

生产部署说明见：

[docs/production-deploy.md](docs/production-deploy.md)

生产上线、升级、回滚和每日巡检手册：

[docs/production-runbook.md](docs/production-runbook.md)

快速启动：

全新安装请使用带初始管理员和默认套餐种子的独立 Compose。运行前须在当前 shell 设置非空的 `MYSQL_ROOT_PASSWORD` 和 `MYSQL_APP_PASSWORD`；`MYSQL_APP_PASSWORD` 必须与 `.env.production` 的 `MYSQL_DSN` 密码一致，缺少变量时 Compose 会拒绝启动。`docker-compose.prod.yml` 的版本化迁移门禁只接受空 schema（依序安装至 v11）、精确受管 v1–v10（顺序升级至 v11）或精确受管 v11（no-op）；迁移成功后才启动 API，并关闭 GORM `AutoMigrate` 与启动种子写入。未知历史结构会被拒绝，API 保持停止。升级时须沿用旧部署的 Compose 项目名（命名卷名包含项目名前缀），并确认 `.env.production` 的 `MYSQL_DSN` 指向该 Compose 内的 `mysql:3306/saas_billing`；独立 MySQL 应使用 `docker-compose.managed.yml`。阅读[生产部署说明](docs/production-deploy.md)中的数据库迁移章节后，再选择适合目标库的入口。

```bash
cp .env.production.example .env.production
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml up -d --build
```

上线前请务必修改：

- `JWT_SECRET`
- `AES_SECRET`
- 新数据库首次创建管理员时设置 `INITIAL_ADMIN_PASSWORD`
- MySQL 密码
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
- 商家自有收款账户对账和直连能力完善
- 支付宝/微信真实退款接口
- 商品分类和图片上传
- 更完整的财务导出和对账
- 生产环境 HTTPS、备份、监控和日志告警

## 联系合作

商务合作与开发交流：long1417341@gmail.com
