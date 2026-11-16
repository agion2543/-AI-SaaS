# 生产部署说明

本文档用于把当前项目部署到云服务器或内网生产环境。当前方案采用：

- 后端：Go Gin
- 前端：Vue3 + Nginx 静态部署
- 数据库：MySQL 8.0
- 编排：Docker Compose

## 1. 服务器准备

建议最低配置：

- CPU：2 核
- 内存：4 GB
- 磁盘：40 GB+
- 系统：Ubuntu 22.04 LTS / Debian 12 / CentOS Stream 均可
- 软件：Docker、Docker Compose Plugin、Git

开放端口：

- `80`：前端和 API 反向代理
- `443`：后续 HTTPS
- 不建议公网开放 `3306`

## 2. 拉取项目

GitHub 仓库必须保留目录结构，例如：

```text
cmd/server/main.go
internal/router/router.go
web/src/router/index.js
sql/migrations/20260430_add_audit_logs.sql
```

如果通过 GitHub 网页上传，请上传整个文件夹或使用 Git，不要把所有文件平铺到仓库根目录。

```bash
git clone https://github.com/agion2543/-AI-SaaS.git
cd -AI-SaaS
```

## 3. 配置生产环境变量

复制示例文件：

```bash
cp .env.production.example .env.production
```

必须修改：

```env
APP_ENV=production
APP_URL=https://api.example.com
FRONTEND_URL=https://example.com
JWT_SECRET=请替换为长随机字符串
AES_SECRET=必须替换为32位随机字符串
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=请替换为初始管理员强密码
MYSQL_DSN=saas_user:数据库密码@tcp(mysql:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local
ALIPAY_NOTIFY_URL=https://api.example.com/api/v1/payments/callback/alipay
ALIPAY_RETURN_URL=https://example.com/payment/return
```

同时创建 Compose 使用的数据库密码：

```bash
export MYSQL_ROOT_PASSWORD='请替换为root密码'
export MYSQL_APP_PASSWORD='请替换为应用数据库密码'
```

也可以把这两个变量放到服务器的安全环境变量管理里，不建议提交到 GitHub。

## 4. 新数据库首次安装

仅将下面的独立 Compose 文件用于**有意创建全新数据库**的安装。它不挂载 `sql/init.sql`，使用专属卷 `saas_fresh_install_v1_mysql_data`，先等待 MySQL 健康，再运行版本化迁移；只有迁移成功后才启动 API。该路径将应用侧 `DB_AUTO_MIGRATE` 设为 `false`，避免 GORM AutoMigrate 成为第二个 schema 管理入口。不要把它指向任何已有数据库或旧数据卷；旧库升级仍不支持。

```bash
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml up -d --build
```

检查迁移器成功退出后，API 才会启动：

```bash
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml ps -a
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml logs schema-migrate
```

查看后端日志：

```bash
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml logs -f backend
```

存活与就绪检查：

```bash
curl http://127.0.0.1/health
curl -i http://127.0.0.1/ready
```

`/health` 仅确认 HTTP 进程有响应，不依赖数据库；`/ready` 会 ping 当前数据库连接，数据库不可用时返回 HTTP 503，且不返回 DSN、主机名或凭据。

`/health` 预期返回：

```json
{"code":0,"message":"success","data":{"status":"ok"}}
```

数据库可用时 `/ready` 预期返回：

```json
{"code":0,"message":"success","data":{"status":"ready","checks":{"database":"ready"}}}
```

数据库不可用时，`/ready` 返回 HTTP 503 和 `data.status: "not_ready"`。

## 5. 域名和 HTTPS

推荐用云厂商安全组开放 `80/443`，再使用 Nginx Proxy Manager、Caddy 或 Certbot 配置 HTTPS。

上线后应把：

```env
APP_URL=https://你的后端域名
FRONTEND_URL=https://你的前端域名
ALIPAY_NOTIFY_URL=https://你的后端域名/api/v1/payments/callback/alipay
ALIPAY_RETURN_URL=https://你的前端域名/payment/return
```

全部改成真实 HTTPS 地址。

## 6. 数据库迁移

**不要**把 `sql/migrations/*.sql` 或 `migrations/*.sql` 按文件名批量执行。当前没有可信的旧版数据库基线或版本记录，旧库升级流程尚未验证；生产环境若需升级旧库，应暂停该步骤，先按
[数据库迁移现状与安全边界](database-migration-status.md)取得对应旧版本的 schema-only 基线并制定经隔离副本验证的升级方案。

新安装应使用 `docker-compose.new-install.yml`：它不会挂载 `sql/init.sql` 或历史 SQL 目录，迁移器从空数据库应用版本 `schema-v1-candidate-20261113`，API 随后只运行初始数据种子逻辑，不执行 GORM AutoMigrate。候选快照来自 HEAD `15274c8` 加 2026-11-13 生成时的本地未提交模型改动，批准范围仅限今后的全新安装；它不代表任何历史数据库结构。

`docker-compose.prod.yml` 保持原样，以免改变既有部署与数据卷的启动行为；它仍使用 `sql/init.sql` 和应用侧 GORM AutoMigrate，**不是**上面新 v1 首装流程的替代命令。任何已有或不确定的数据库都不能直接套用新安装流程。迁移器会拒绝未知非空结构、部分初始化、版本/校验和不符或结构漂移；不得删除账本来绕过拒绝。旧库升级仍未支持、未验证。具体限制见[数据库迁移现状与安全边界](database-migration-status.md)。

更完整的首次部署、升级、回滚和每日巡检流程见：

```text
docs/production-runbook.md
```

## 7. 备份

建议每天备份 MySQL：

```bash
docker exec saas-mysql-prod mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" saas_billing > backup-$(date +%F).sql
```

至少保留：

- 最近 7 天每日备份
- 最近 8 周每周备份
- 每次上线前手动备份

## 8. 上线前检查清单

- `.env.production` 没有提交到 GitHub
- `JWT_SECRET` 已替换为强随机值
- `AES_SECRET` 是 32 位随机字符串
- `INITIAL_ADMIN_PASSWORD` 已替换为强密码，且登录后台后已再次修改管理员密码
- MySQL `3306` 未暴露到公网
- 支付宝回调地址为公网 HTTPS
- 已开启数据库备份
- 已确认平台端、商家端、顾客端核心流程
- 已确认操作审计能记录退款、开通订阅、审核收款等动作
- 已按 `docs/production-runbook.md` 做过一次发布前演练和回滚预案确认

## 9. 当前支付建议

**重要：以下是目标建议，不代表当前代码已统一按此运行。** 代码库没有独立的预约模型、预约接口或预约页面；目前只有订单备注提示可填写“预约时间”，所以预约是否预付、何时付款还没有实现的产品规则。

现有点单代码仍保留两种不同路径：订单处于 `pending` 时，创建/重试接口可以生成支付宝支付入口，并由支付回调记录渠道确认；而商户二维码路径会把顾客的“已转账”标记设为 `payment_confirming`，只有商户手动点确认后才写入本地收款记录。顾客的标记本身不是到账凭证，后一路径不做渠道自动验账。**当前决策：先保留现有支付宝预付路径，不限制、停用或改造底层支付流程；优先优化其他方向，之后再单独评估支付模式。**保留现状不代表已完成真实商户或支付验证；实际启用前仍需按门店场景验证。

前期建议：

- 平台订阅收入进入平台账户
- 顾客扫码点单收入进入商家自己的支付宝/微信账户
- 平台只做订单记录、流水记录和 SaaS 服务收费
- 商家收款配置默认使用 `direct` 模式，`platform` 和 `service_provider` 暂作为后续规模化预留模式

在上述规则和现有路径完成统一前，不要把本节的建议当作已经通过真实收款或预约流程验收；平台不应代收、归集、托管或清分顾客点单款。

不建议早期使用“平台统一收款再手动分给商家”，这可能涉及资金清分、二清和对账合规风险。后期如需平台统一收款，应接入官方服务商/分账能力。

上线 MVP 时建议把后台“收款策略说明”配置为：平台收 SaaS 订阅费，顾客点单款走商家自有收款账户，平台仅做订单记录、退款记录、运营巡检和人工对账。
