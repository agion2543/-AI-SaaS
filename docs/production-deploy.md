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

## 4. 启动生产服务

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

查看容器：

```bash
docker compose -f docker-compose.prod.yml ps
```

查看后端日志：

```bash
docker logs -f saas-backend-prod
```

健康检查：

```bash
curl http://127.0.0.1/health
```

预期返回：

```json
{"code":0,"message":"success","data":{"status":"ok"}}
```

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

首次启动会执行 `sql/init.sql`。已有数据库升级时，需要按时间顺序执行：

```bash
sql/migrations/*.sql
```

生产环境建议使用专门迁移工具，后续可接入 `golang-migrate`。

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
- 管理员默认密码已修改
- MySQL `3306` 未暴露到公网
- 支付宝回调地址为公网 HTTPS
- 已开启数据库备份
- 已确认平台端、商家端、顾客端核心流程
- 已确认操作审计能记录退款、开通订阅、审核收款等动作

## 9. 当前支付建议

前期建议：

- 平台订阅收入进入平台账户
- 顾客扫码点单收入进入商家自己的支付宝/微信账户
- 平台只做订单记录、流水记录和 SaaS 服务收费

不建议早期使用“平台统一收款再手动分给商家”，这可能涉及资金清分、二清和对账合规风险。后期如需平台统一收款，应接入官方服务商/分账能力。
