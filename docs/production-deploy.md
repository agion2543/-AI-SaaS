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
JWT_SECRET=replace-with-a-long-random-jwt-secret
AES_SECRET=replace-with-a-32-byte-random-key
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=replace-with-a-strong-initial-admin-password
MYSQL_DSN=saas_user:replace-with-a-strong-db-password@tcp(mysql:3306)/saas_billing?charset=utf8mb4&parseTime=True&loc=Local
ALIPAY_NOTIFY_URL=https://api.example.com/api/v1/payments/callback/alipay
ALIPAY_RETURN_URL=https://example.com/payment/return
```

`AES_SECRET` 必须恰好为 32 字节，程序将其 UTF-8 字节原样用作 AES-256 密钥；不要填入 64 个十六进制字符并期待程序解码。该密钥必须跨重启和升级保持不变：系统配置密文依赖当前密钥，直接更换会使已有密文无法读取。当前项目没有密钥环或自动轮换流程；轮换前必须另行设计并在隔离副本验证“旧密钥解密、用新密钥重加密、核验后切换”的迁移。密钥不匹配时，后台加密配置读取会失败并返回错误，不会把无法解密的密文当明文返回。

同时创建 Compose 使用的数据库密码：

```bash
export MYSQL_ROOT_PASSWORD=replace-with-a-strong-mysql-root-password
export MYSQL_APP_PASSWORD=replace-with-a-strong-mysql-app-password
```

也可以把这两个变量放到服务器的安全环境变量管理里，不建议提交到 GitHub。
生产部署和全新安装 Compose 都要求这两个变量非空；未设置时 Compose 会直接报错，不再回退到内置占位密码。`MYSQL_APP_PASSWORD` 必须与 `.env.production` 中 `MYSQL_DSN` 的密码一致。

在生产 Compose 启动前，从仓库根目录运行只读预检：

```bash
python3 scripts/preflight-production-deploy.py
```

它会检查 `.env.production` 的必需配置和应用/前端/支付回调 URL 格式、当前 shell 中的 `MYSQL_ROOT_PASSWORD`/`MYSQL_APP_PASSWORD`，并确认 `MYSQL_DSN` 指向本项目的 `saas_user@mysql:3306/saas_billing` 且密码一致。检查不会输出密钥、访问支付服务、连接数据库或启动容器；未通过时先修正配置再继续。

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

生产、托管数据库和全新安装 Compose 配置中的 backend healthcheck 使用 `/ready`，因此数据库不可达时容器健康状态不会仍显示为正常。Docker/Compose 健康状态本身不发送告警，也不代表已有外部监控或自动恢复；上线方仍需配置 `/health`、`/ready` 和前端入口的独立探测、阈值、通知接收人及故障演练。

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

`docker-compose.prod.yml` 保留原来的 `mysql_prod_data` 卷键，但移除了 `sql/init.sql` 与历史 SQL 挂载；它先运行 `/app/schema-migrate`，只有迁移成功后才启动 API，并显式设置 `DB_AUTO_MIGRATE=false`、`INITIAL_DATA_SEED_ENABLED=false`。Compose 命名卷会附加项目名前缀，因此升级时必须沿用旧部署的同一个 `-p` 项目名（若旧命令未指定 `-p`，应从同一项目目录运行）；更换项目名会创建另一个卷，不能把它当成已迁移的旧数据库。未知历史结构会被拒绝，API 不启动，也不会回退到 GORM `AutoMigrate`。此入口只接受真正空 schema（依序安装 v1→v10）、精确受管 v1–v9（顺序升级至 v10）或精确受管 v10（no-op）；全新安装若需要管理员和默认套餐种子，请使用上面的 `docker-compose.new-install.yml`。**运行此 Compose 前，必须确认 `.env.production` 的 `MYSQL_DSN` 指向本项目内的 `mysql:3306/saas_billing`，且密码与 `MYSQL_APP_PASSWORD` 相同；同时在 Compose 的环境中提供非空的 `MYSQL_ROOT_PASSWORD` 和 `MYSQL_APP_PASSWORD`。连接独立 MySQL 时改用 `docker-compose.managed.yml`。**旧库升级仍未支持、未验证；不得删除账本绕过拒绝。具体限制见[数据库迁移现状与安全边界](database-migration-status.md)。

### 空 schema 或精确受管数据库的版本化部署

若数据库使用生产 Compose 自带的 MySQL 与 `mysql_prod_data` 卷，可运行 `docker-compose.prod.yml`：迁移服务等待 MySQL healthy，backend 等待迁移服务成功退出。真正空 schema 会安装 v1→v10，精确受管 v1–v9 会升级至 v10，精确受管 v10 为 no-op；非空未知旧库、部分迁移、额外对象或结构漂移均失败关闭，backend 不会启动。启动前须提供非空的 `MYSQL_ROOT_PASSWORD`、`MYSQL_APP_PASSWORD`；`.env.production` 中的 `MYSQL_DSN` 必须指向 `mysql:3306/saas_billing`，其密码与 `MYSQL_APP_PASSWORD` 一致。独立 MySQL 必须选用下述 managed Compose。更新时沿用原 Compose 项目名，确保项目名前缀对应的旧 MySQL 卷被复用；该入口关闭启动种子写入，不能替代需要初始管理员/默认套餐的新安装流程。

`docker-compose.managed.yml` 是连接独立 MySQL 的 opt-in 路径：它不创建 MySQL、不挂载数据卷或历史 SQL，也不运行 GORM AutoMigrate。它与生产 Compose 都设置 `INITIAL_DATA_SEED_ENABLED=false`，避免迁移启动时改写 `super_admin` 角色、创建初始管理员或插入默认会员套餐/商家计划；应用配置虽默认开启种子，Compose 会显式覆盖。managed 迁移器虽接受空 schema，但不会创建初始管理员或默认套餐；首次安装需要这些种子数据时应使用上面的 `docker-compose.new-install.yml`。先在 `.env.production` 把 `MYSQL_DSN` 明确设为容器可达的目标 MySQL；数据库本身须已创建，账号须具备迁移所需 DDL 权限。它只接受空 schema（安装 v1→v10）或 checksum、账本和结构指纹精确匹配的受管 v1–v9（逐级升级到 v10）；精确受管 v10 会作为 no-op。非空未知旧库、部分/中断迁移或任何结构漂移都拒绝，迁移失败时 API 不会启动。

在切换部署前先备份并停止旧应用对目标库的写入；不要同时运行两套 Compose 连接同一数据库。不要把新安装 Compose 的专属卷与既有数据库混用，也不要将原始历史 SQL 批量执行。未知旧库升级依旧不支持、不应尝试猜测。若版本化迁移在 DDL 中途失败，可能留下 `applying` 状态或部分对象；不要删账本或手工补结构，应停止操作并恢复经验证备份，或仅在隔离副本中排查。

```bash
# 先确认 .env.production 的 MYSQL_DSN 指向预期数据库，而不是默认的 mysql:3306
docker compose -p saas-managed -f docker-compose.managed.yml up -d --build
docker compose -p saas-managed -f docker-compose.managed.yml ps -a
docker compose -p saas-managed -f docker-compose.managed.yml logs schema-migrate
```

该 Compose 含有绑定宿主机 `80` 端口的前端；切换时不可与另一套同端口前端同时启动。新库首次安装若使用项目自带 MySQL 和专属空卷，应继续使用上面的 `docker-compose.new-install.yml`。

更完整的首次部署、升级、回滚和每日巡检流程见：

```text
docs/production-runbook.md
```

## 7. 备份

本节是目标策略，不代表生产备份或密钥已配置。目标为每日备份并滚动保留最近 30 天；发布前的手动备份计入该窗口。确需延长保留时，应由数据负责人明确批准并记录负责人、理由和到期日。仓库提供手动预览与令牌确认的加密备份清理器，但没有自动排程、自动删除或内置备份告警。

`scripts/backup-mysql.sh` 与 `scripts/backup-mysql.ps1` 需要安装 age 并设置 `AGE_RECIPIENTS_FILE`（公开公钥清单），成功时生成 `.sql.age`，拒绝覆盖同名备份，并清理临时明文；Windows 脚本的明文临时目录仅授予当前用户访问。Bash 使用 `DB_PASSWORD` 为 `mysqldump` 设置 `MYSQL_PWD`，并在启动 age 时移除 `MYSQL_PWD` 与 `DB_PASSWORD`；PowerShell 只从 `MYSQL_PWD` 环境读取数据库密码、不接受命令行密码参数，并在运行 age 前暂时清除这两个环境变量、随后恢复调用者原值。私钥 identity 必须保存在仓库和备份目录之外并留有离线恢复副本。`scripts/backup-integrity.py` 可为 `.sql` 或 `.sql.age` 创建和核对 SHA-256 清单；密文清单只校验字节内容，不是签名，不证明备份来源可信或能恢复。清单应与备份分开保存。

生产环境若启用每日备份，部署方必须配置计划任务并接入日志/告警：收集脚本 stdout、stderr 与退出码，对非零退出、预期周期内缺少新 `.sql.age` 或清单、以及清单 `verify` 失败告警。脚本成功提示只表示 dump/加密命令成功结束并发布了非空密文，不证明该密文可解密或可恢复。

手动创建并核对清单的示例（先确认 `BACKUP_DIR` 是本次要检查的备份目录）：

```bash
set -euo pipefail
RUN_ID="$(date +%Y%m%d-%H%M%S)-$$"
BACKUP_DIR="./backups/$RUN_ID"
MANIFEST="./backup-manifests/$RUN_ID.json"
AGE_RECIPIENTS_FILE="/secure/path/backup-recipients.txt" # 外部受控文件，仅含公开 age recipients
mkdir -p "$(dirname "$MANIFEST")"
AGE_RECIPIENTS_FILE="$AGE_RECIPIENTS_FILE" BACKUP_DIR="$BACKUP_DIR" bash scripts/backup-mysql.sh
python3 scripts/backup-integrity.py create --backup-dir "$BACKUP_DIR" --manifest "$MANIFEST"
python3 scripts/backup-integrity.py verify --backup-dir "$BACKUP_DIR" --manifest "$MANIFEST"
```

每次备份使用独立目录，之后核对时继续指向同一个目录和清单；该清单会记录目录内完整的 `.sql`/`.sql.age` 文件清单，不能把后续备份追加进同一个目录，否则旧清单会正确报出文件清单变化。PowerShell 调用前同样要在安全环境中设置 `AGE_RECIPIENTS_FILE`；不要把私钥或数据库密码写进命令参数、仓库或清单。

可用 `health` 对一个明确的备份目录和对应清单做单次检查；最大年龄必须由调用方显式设置，不存在默认周期：

```bash
: "${BACKUP_MAX_AGE_SECONDS:?Set the caller-selected freshness threshold in seconds}"
python3 scripts/backup-integrity.py health \
  --backup-dir "$BACKUP_DIR" \
  --manifest "$MANIFEST" \
  --max-age-seconds "$BACKUP_MAX_AGE_SECONDS"
```

此命令要求至少一个直属 `.sql.age` 文件；以其中最新文件的本地修改时间检查显式阈值，并核验目录内完整 SQL/密文清单及 SHA-256。检查期间若文件身份或元数据变化也会失败。外部任务计划可依据退出码（0 表示这次检查通过，非零表示缺少、过期或不匹配）处理；本仓库不会创建计划任务或告警。它使用文件系统时间而非签名时间戳，且不解密、不导入，不能证明备份可恢复。

保留清理默认为 dry-run，先检查输出的确切候选清单。只有在确认这些文件可删后，才用预览中给出的令牌再次执行；目录、候选集合或候选元数据变化会让令牌失效。工具只清理直属的过期 `.sql.age` 常规文件，保留至少一份密文，不删除旧明文 `.sql`：

令牌确认后，脚本会先把每个候选原子移入同目录下权限受限的 `.cue-backup-retention-quarantine-*` 临时目录，核对实际移入的文件后才删除。若校验失败或进程中断，隔离目录会保留，后续清理会拒绝继续；请先人工检查其中的密文，在原文件名尚未被占用时恢复到备份目录，再移除空隔离目录，不要直接递归删除隔离目录。

```bash
python3 scripts/backup-retention.py --backup-dir "$BACKUP_DIR"
python3 scripts/backup-retention.py --backup-dir "$BACKUP_DIR" --apply --confirm-token '替换为预览打印的令牌'
```

`bash scripts/test-backup-mysql.sh`、`bash scripts/test-backup-integrity.sh` 和 `bash scripts/test-backup-retention.sh` 可在合成文件上检查 mock age 调用、失败清理、不覆盖、SHA-256、确认令牌和删除边界；这些 mock 不是真实加密。Windows PowerShell 备份另有 `scripts/test-backup-mysql.ps1`。`bash scripts/validate-schema-lifecycle.sh` 的临时 MySQL 合成流程也不等于真实 age 恢复演练。

要在本地重复运行合成加密备份恢复测试，使用以下命令：

```bash
bash scripts/test-backup-restore-age.sh
```

要求在 **Linux 非 root** 环境预先安装本地 `age`、`age-keygen`、MySQL 8.x 的 `mysqld`/`mysql`/`mysqladmin`、`ss`、`ps` 及常见 Linux 命令行工具；脚本不会联网、下载或安装工具，缺少依赖时会在创建临时目录前报错。它用临时生成的 identity/recipient、mock `mysqldump` 和合成 SQL 调用仓库 Bash 备份脚本，再用真实本地 age 解密、逐字节比较，并导入全新临时 MySQL；MySQL 使用临时 Unix socket、关闭 TCP，查询必须精确匹配一条合成记录。脚本拒绝调用环境中非空的 `MYSQL_DSN`、`MYSQL_PWD` 或 `DB_PASSWORD`，并清理本次创建的临时文件和进程；若无法安全停止 MySQL，会保留其 datadir 并报告路径。

该演练只证明这组本地合成数据通过了脚本覆盖的加密、解密和导入路径，不读取现有备份，也不验证生产备份、生产 identity/密钥保管、异地副本、Windows PowerShell、恢复时间目标或生产恢复流程；不能替代生产恢复验证。不得用真实客户数据做测试。

## 8. 上线前检查清单

- `.env.production` 没有提交到 GitHub
- `JWT_SECRET` 已替换为强随机值
- `AES_SECRET` 是恰好 32 字节的强随机值，并且与创建密文时使用的值一致
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
