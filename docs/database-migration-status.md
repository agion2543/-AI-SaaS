# 数据库迁移现状与安全边界

## 原有生产 Compose 路径（保留，不作为 v1 新安装入口）

新安装只适用于**空的 MySQL 数据卷**。生产 Compose 将 `sql/init.sql` 挂载为 MySQL 初始化目录顶层的 `001_init.sql`；MySQL 镜像只在首次初始化空数据目录时处理这些初始化文件。随后应用启动时，`internal/bootstrap/app.go` 依次连接数据库、对当前模型执行 GORM `AutoMigrate`，再运行 `seedInitialData`。

Compose 还把 `sql/migrations` 挂载到初始化目录下的 `migrations/` 子目录。它不是顶层初始化 SQL 文件，因此不会因此自动逐个执行；根目录的 `migrations/` 也没有挂载到生产 Compose。已有数据卷不会因容器重启而重新执行 `sql/init.sql`。

以上是原有 `docker-compose.prod.yml` 的行为。为避免改变未知部署和数据卷，其配置保持不变；此路径仍由 `sql/init.sql` 与 GORM AutoMigrate 管理结构，不应与下述版本化 v1 新安装流程混用。此前的隔离 MySQL 测试**不等同于**从旧版数据库升级，也不能证明旧库兼容或历史 SQL 可安全重跑。

## 旧数据库升级：目前不支持、不应猜测执行

仓库没有可信的旧版数据库结构基线，也没有迁移版本表记录各 SQL 是否已执行。历史迁移分散在 `sql/migrations/` 和 `migrations/` 两处，文件名时间顺序不是可靠的依赖顺序。当前无法判断一套既有数据库缺少哪些表、列、索引、约束或数据转换。因此，旧库升级流程目前属于**未支持、未验证**；不得把迁移文件按通配符或按文件名批量执行当作升级方案。

典型依赖和风险包括：

| 迁移 / 主题 | 已确认的依赖或风险 |
| --- | --- |
| `20260428_add_customer_lead_followup.sql` 与 `20260428_add_customer_leads.sql` | 文件名字典序会先遇到 follow-up 脚本，但它要修改 `customer_leads`；建表脚本又依赖先有 `merchants` 和 `stores`。仅按名字执行会失败。follow-up 脚本还会更新既有线索记录。 |
| `20260428_add_promotions.sql`、`20260510_add_referral_coupons.sql` | 外键依赖商家/门店；优惠券还依赖 `share_campaigns`。执行前必须先确定目标数据库确有对应表及兼容结构。 |
| 两条商家订阅历史迁移（`20260428_add_merchant_subscriptions.sql`、`20260429_add_merchant_subscription.sql`） | 都会改动订阅相关字段，且对商家/订单的预期结构不同；与当前模型的 `AutoMigrate` 也可能重叠，不能假定可以串行叠加。 |
| 多数旧版 `ALTER TABLE ... ADD COLUMN/INDEX` | 有些语句没有“已存在”保护，第二次执行会因列或索引重复而失败。另一个 `migrations/` 目录中的订单评价迁移也有此风险。 |
| 含 `UPDATE` / `INSERT` 的历史 SQL | 例如商家订阅同步、待审核商家激活、优惠字段转换及种子写入会改动已有业务值；重复执行可能重写状态、计划字段或初始数据，不能只凭文件名判断无副作用。 |
| `20261113_add_refund_reconciliation_audits.sql` | 除建表外还会删除并重建数据库触发器。它有意阻止审计记录被更新/删除；重跑会改变触发器安装时点，不能在仍有业务流量时随手执行。 |
| `sql/init.sql` | 含重复键更新：重复手动执行可能改写管理员角色信息及会员套餐价格、额度、描述。不要把它当作已有库的升级脚本。 |

个别 2026-11-13 脚本做了信息架构检查或动态 DDL，且已在特定空合成结构上测试过；这不代表整个历史目录幂等，更不代表这些脚本可直接用于生产旧库。

## 现在可以与不可以做什么

**可以：**在一次性、隔离、无真实商家/支付数据的空测试 MySQL 上验证版本化 v1 新安装流程；运行默认 Go 测试。若专门验证保留的旧 Compose 行为，也只可在隔离合成库中进行。MySQL 专用集成测试只能按运维手册所列规则使用临时目录、Unix socket、关闭 TCP 的合成实例。

**不可以：**把 `sql/migrations/*.sql` 或 `migrations/*.sql` 当作可执行升级批次；在生产库或任何既有业务库上试跑/重跑历史迁移；对已有库手动重放 `sql/init.sql`；把 v1 候选基线当作旧版结构或升级方案；把历史脚本的合成测试当作旧版升级验收；或擅自在保留的旧 Compose 流程中移除 `AutoMigrate`。目前不应对真实商家、支付数据或既有数据库做迁移验证。

## 长期单一权威与旧库升级

旧库升级前，先取得**具体旧版本/提交对应的 schema-only 基线**（或由该旧版本在一次性空库中生成的结构），明确该版本已执行的变更；不需要真实商家数据。随后在隔离副本上逐项核对旧结构、升级结果、数据不变量、回滚/恢复办法，并形成可重复的测试。没有该基线就继续把旧库升级标为阻塞，而不是试跑猜测的 SQL 顺序。

新的空库入口已使用仓库内版本化迁移器和账本，但历史 SQL 的双目录仍未整理。旧库升级前仍须取得具体历史版本的结构基线，并在隔离副本验证迁移覆盖、数据不变量与恢复方案；本次改动没有给旧库增加兼容路径。

## 新安装专用 schema v1 基线
已生成 [`sql/baselines/schema-v1-candidate-20261113.sql`](../sql/baselines/schema-v1-candidate-20261113.sql)：它基于 HEAD `15274c8` 加生成时的本地未提交工作树模型，在隔离的空 MySQL 8.0.46 中调用当前 `autoMigrate`，并应用现有退款对账审计迁移以包含两条只追加触发器。快照仅含结构，不含应用种子或业务数据；已导入第二个全新隔离实例，并核对表、列、索引、外键及触发器目录一致。

该快照现已批准为**未来全新安装**的 v1 schema 基线，准确标识为 `schema-v1-candidate-20261113`；其来源是 HEAD `15274c8` 加 2026-11-13 生成时的本地未提交模型改动，并不代表 HEAD 单独对应的历史数据库结构。批准不扩展到任何既有数据库，不构成旧库升级支持或验证。保留快照文件原样及其 checksum；v2 只接受本段所述的精确 managed v1，不尝试改写旧版文件或账本。


## 新安装执行流程与拒绝条件

未来全新安装使用 `docker-compose.new-install.yml`，命令如下：

```bash
docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml up -d --build
```

该 Compose 使用专属数据库卷，不挂载 `sql/init.sql`，等待 MySQL healthy 后执行容器内 `/app/schema-migrate`；后端依赖迁移服务成功完成，并设置 `DB_AUTO_MIGRATE=false`。应用启动后仍会写入其初始角色、管理员和套餐种子数据。

v1 基线版本仍为 `schema-v1-candidate-20261113`，SHA-256 为 `d4277d6999a8c5fade3edda12e6d7d9bfbf7874c34a0b0406d03573b68609006`。最新安装入口 `/app/schema-migrate` 先在完全空库安装此 v1，再应用 v2、v3、v4、v5 与 v6 五个严格校验的增量；重复调用只有在最新账本与结构指纹全部精确匹配时才返回已管理。旧的 `ApplyCandidate` 仍只校验 v1，不会把 v2、v3、v4、v5、v6 或其他结构当作 v1。

增量版本 `schema-v2-subscription-manual-review-20261113` 的 SHA-256 为 `48ef3ec703a314a213107af20ad415e80dc4aa27cdccaf3349cdc9ab53ea5e26`。它只可从准确的托管 v1 账本、校验和与结构指纹升级；升级后再分别验证原 v1 子结构指纹和完整 v2 指纹。v2 增加 `merchant_subscription_manual_reviews` 与 `merchant_subscription_manual_review_audits` 两张专用表、唯一订单键及四条阻止 UPDATE/DELETE 的数据库触发器，不修改 v1 快照或其他表。只有空库的新安装或完全匹配的 v1 可以推进；未知旧库、部分安装、额外对象、checksum/结构漂移或 `applying` 状态继续拒绝且不自动修复。一次中断留下的 v2 `applying` 状态需要从干净隔离库重新开始或恢复备份，不能删除账本后硬重试。

增量版本 `schema-v3-admin-auth-version-20261114` 的 SHA-256 为 `1398762fd14d351eeb61e2b26f98513a18401bbc77e6da677b91e2f5481f23d4`。它只从准确托管的 v2 增加 `admin_users.auth_version BIGINT UNSIGNED NOT NULL DEFAULT 1`，不更改已批准的 v1 基线或 v2 表/触发器；升级后逐一验证 v1、v2 原始子结构指纹（排除新增列）及完整 v3 指纹。管理员密码恢复、状态/角色分配变化和角色权限变化会原子递增此计数器，使旧 token 不会在状态恢复后复活。

增量版本 `schema-v4-otp-safety-20261114` 的 SHA-256 为 `90aba9d894e7e6af5ecc9904745501472ceb1c20f74248fe360649eca371dde0`。它只从精确托管的 v3 增加 nullable `verification_codes.code_hash CHAR(64)`（保存绑定手机号和用途的 HMAC-SHA-256 摘要）及 `failed_attempts INT UNSIGNED NOT NULL DEFAULT 0`。迁移会把旧 `code` 列内容覆盖为 `REDACTED`、将新摘要列设为不可匹配标记并作废所有旧码；升级期间已有验证码必须重新申请。新增列不改变原 `code` 列结构；迁移后核验 v1、v2、v3 的原始结构指纹（排除 v4 新列）及完整 v4 指纹，v1-v3 快照和校验和保持不变。手机号流程限为每手机号 60 秒冷却、滚动 1 小时最多 5 次发送、每个验证码 10 分钟有效且最多 5 次错误尝试；正确验证码一次性消费。短信日志不再输出验证码。

增量版本 `schema-v5-email-otp-safety-20261115` 的 SHA-256 为 `59a48027c17bc375c3c1d15e22c0db966b3454de8980e9416c3c65d1eff69dbc`。它只从精确托管的 v4 增加 nullable `email_verification_codes.code_hash CHAR(64)`（保存绑定邮箱和用途的 HMAC-SHA-256 摘要）及 `failed_attempts INT UNSIGNED NOT NULL DEFAULT 0`。迁移会把旧 `code` 覆盖为 `REDACTED`、写入不可匹配摘要并作废所有旧邮件验证码；升级后必须重新申请。迁移保留原列结构并验证 v1-v4 历史指纹与完整 v5 指纹。邮箱流程按每邮箱限制为 60 秒冷却、滚动 1 小时最多 5 次；验证码有效 10 分钟、最多 5 次错误尝试，成功后一次性消费。当前未接入邮件服务：开发环境用固定测试码 `123456` 模拟且不记录验证码或邮箱日志；生产、staging 等非开发环境在未配置真实邮件服务时拒绝模拟发送。v5 不连接邮件服务，也不会发出真实邮件。

增量版本 `schema-v6-wholesale-retail-20261116` 的 SHA-256 为 `a925b154315d9d34475e08911fcc0abb25b7986a14c3da11d3bc251f9adc65f0`。它只从精确托管的 v5 增加商品可选销售单位、规格、最低购买量和商家自定义数量档位价 JSON 列，不改写既有订单或支付状态；升级时继续验证 v1-v5 历史结构指纹，完整 v6 账本和商品列指纹通过后才认定已管理。档位阈值与价格由商家自行配置，订单金额由服务端根据商品当前配置重算，订单行保留下单时的单位、规格、基础/实际单价及档位快照。

新安装入口 `ApplyLatest` 依序安装 v1→v2→v3→v4→v5→v6；它仅接受准确托管的 v1/v2/v3/v4/v5/v6，未知旧库、部分安装或漂移结构仍 fail-closed。

历史订阅人工核验是与普通 provider 支付确认分离的流程：后台必须记录管理员对原始凭证及“外部付款确已发生”的明确断言；它把订单标成 `manually_verified`，在专用不可变复核/审计中保存原始条款、证据引用、理由和前后状态，并据所填期限授予订阅权益。它不调用支付渠道、不生成 provider `PaymentRecord`、`PaidAt` 或渠道交易号；普通 provider-confirmed 收入统计只统计原有 `paid` 状态，不把 `manually_verified` 混入。该状态依赖管理员确实核对外部原始证据，系统不会替其验证。

默认 `docker-compose.prod.yml` 保持原样且仍保留 GORM AutoMigrate，避免改变既有安装行为；不要因其存在就认为旧库升级已支持。v1 新安装流程本身、真实部署和旧数据库升级均须各自验证；本地合成测试不能替代真实发布演练。MySQL 集成测试只允许使用 `/tmp/cue-schema-migrate-*` 下的临时 Unix socket，且测试数据库须以 `cue_schema_migration_it` 开头；集成用例拒绝 TCP DSN。

### 可重复的本地全流程检查
需要在本地验证批准的 v1 → v2 → v3 → v4 → v5 → v6 新安装、合成订阅人工复核、备份恢复及只读后台读数时，运行 `bash scripts/validate-schema-lifecycle.sh`。该脚本使用本次新建的临时 MySQL 8 datadir 和 socket-only 实例，不使用 Docker、`MYSQL_DSN`、`.env` 或生产备份脚本的默认 TCP 目标。完整覆盖与限制见[本地数据库生命周期验证](local-schema-lifecycle-validation.md)；它仍不验证旧库升级或生产备份配置。
