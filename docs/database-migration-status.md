# 数据库迁移现状与安全边界

## 默认生产 Compose：版本化迁移门禁

`docker-compose.prod.yml` 保留 `mysql_prod_data` 卷键，但 Docker Compose 会按项目名给命名卷加前缀；只有沿用旧部署的同一个项目名才能复用原数据卷。它移除了 `sql/init.sql` 与历史 SQL 挂载。MySQL healthy 后，Compose 运行 `/app/schema-migrate`；只有迁移容器成功退出，backend 才启动。迁移器与 backend 读取 `.env.production`，其中 `MYSQL_DSN` 必须指向该 Compose 的 `mysql:3306/saas_billing`，密码须与 `MYSQL_APP_PASSWORD` 一致；独立 MySQL 应使用 `docker-compose.managed.yml`。backend 显式设置 `DB_AUTO_MIGRATE=false` 与 `INITIAL_DATA_SEED_ENABLED=false`，因此既不执行 GORM `AutoMigrate`，也不会在升级启动时改写初始角色、创建初始管理员或补默认套餐/计划。

迁移器只接受真正空 schema（按 v1→v17 安装）或精确受管 v1–v16（顺序升级至 v17）；精确受管 v17 为 no-op。v9 只新增五张桌位点餐表；v10 只向 `store_tables` 和 `table_order_details` 追加标签列，不改写 v1–v9 的 schema、迁移记录、校验和或历史结构指纹；v11 新增 `order_items` 明细表并向 `orders` 追加 `channel` 列（store_qr / table / share，桌台历史订单回填为 table），不改写 v1–v10 的 schema、迁移记录、校验和或历史结构指纹；v12 新增 AI 成本控制五张表并向 `merchant_ai_usage_logs` 追加 `store_id`/`cached_tokens` 列，不改写 v1–v11 的 schema、迁移记录、校验和或历史结构指纹；v13 新增公告、维护计划、活动目标、活动复盘、AI 建议执行、团购手工核销六张表，不改写 v1–v12 的 schema、迁移记录、校验和或历史结构指纹；v14 向 `platform_expenses` 追加六列（固定/变动、供应商、付款状态、是否经常性、现金/经济成本标志，无 CHECK 约束与索引）并把分类 CHECK 由六类拓宽为八类（新增 `ai_services`、`third_party_services`，约束名不变），同时新增 `platform_time_logs` 与 `platform_income_records` 两张表，不改写 v1–v13 的 schema、迁移记录、校验和或历史结构指纹；v15 只向 `payment_records` 追加 `operator_id`/`operator_role` 两列、向 `refund_records` 追加 `reviewer_id`/`reviewer_role`/`review_reason`/`reviewed_at` 四列（退款复核），不改写 v1–v14 的 schema、迁移记录、校验和或历史结构指纹；v16 只新增 `customer_profiles` 物化表并用历史有效订单回填，不改写 v1–v15 的 schema、迁移记录、校验和或历史结构指纹；v17 只向 `merchants` 追加 `review_reason`/`reviewed_by`/`reviewed_at` 三列（入驻审核），不改写 v1–v16 的 schema、迁移记录、校验和或历史结构指纹。未知旧结构、部分初始化、额外对象、账本/checksum/结构指纹不符均拒绝，API 保持停止且不会回退到 AutoMigrate。旧版 Compose 曾挂载 `sql/init.sql` 并由应用执行 AutoMigrate，因此保留下来的卷可能属于未知历史结构；迁移服务拒绝它是预期的 fail-closed，不会删除或重置该卷。全新安装若需要初始管理员和默认套餐，应使用带专属卷及初始种子的 `docker-compose.new-install.yml`。

这条门禁不代表任意旧库升级已支持或已验证，也不证明历史 SQL 可安全重跑。部署前应备份、停掉旧应用写入，并确认现存数据卷对应的 schema 在受支持范围内；若迁移器拒绝旧库，应保持 API 停止，先取得具体版本的 schema-only 基线并在隔离副本上设计验证方案，不能清账本或套用新安装 Compose。

## 默认开发 Compose：同样走版本化迁移门禁（P0 风险 1 修复）

此前默认 `docker-compose.yml` 是第二条建库路径：它挂载 `sql/init.sql` 并让 backend 以默认开启的 GORM `AutoMigrate` 建 34 张表；之后若同一数据库改走生产/托管 Compose，迁移器会因缺账本、缺触发器、缺 v7/v8 表、多余索引、datetime 精度不同而拒绝，服务无法启动；反向只走 `AutoMigrate` 的部署则根本没有成本表（v7/v8）与退款对账表，相关代码运行时直接报错。此后默认 Compose 与生产/全新安装/托管 Compose 一致，**唯一建库路径是版本化迁移器（v1→v14）**：

- `docker-compose.yml` 增加 `schema-migrate` 服务（与 backend 同一镜像，执行容器内 `/app/schema-migrate`），等待 MySQL healthy 后运行；backend 改为依赖 `schema-migrate` 成功完成（`service_completed_successfully`）后才启动，并显式设置 `DB_AUTO_MIGRATE=false`。同时移除 `sql/init.sql` 挂载（文件保留在仓库中，仅作历史记录，不再由任何 Compose 入口执行），MySQL 增加 healthcheck 与 utf8mb4 字符集/排序规则（与生产 Compose 一致）。
- 开发种子行为不变：默认 Compose 的 backend 显式设置 `INITIAL_DATA_SEED_ENABLED=true`（与 `docker-compose.new-install.yml` 相同），迁移完成后仍由应用写入初始角色、初始管理员与默认套餐/计划；首次创建管理员仍须在 `.env` 中设置至少 12 位的 `INITIAL_ADMIN_PASSWORD`，未设置时服务拒绝完成新库初始化。
- 应用配置默认值同步改为 `DB_AUTO_MIGRATE=false`：不经 Compose 直接运行 `./cmd/server` 的裸机启动也不再默认执行 `AutoMigrate`。裸机启动前须先跑一次版本化迁移器（`go run ./cmd/schema-migrate`，读取同一 `MYSQL_DSN`），或仅对一次性本地实验显式设置 `DB_AUTO_MIGRATE=true`；显式开启只适用于可丢弃的本地实验库，绝不用于任何共享或已部署数据库。单元测试内直接调用的 `AutoMigrate`（SQLite 内存夹具）不受此默认值影响。
- `sql/migrations/*.sql` 与 `migrations/*.sql` 本次未删、未改、未获得任何执行入口：它们仍是历史记录，不是可执行批次；唯一执行建库逻辑的是 `internal/schema/` 迁移器。

**新装流程（默认 Compose）：**空数据卷 → MySQL healthy → `schema-migrate` 按 v1→v11 安装 → backend 启动（`DB_AUTO_MIGRATE=false`）→ 应用种子写入初始数据。`schema-migrate` 与 backend 读取同一个 `.env` 的 `MYSQL_DSN`，Compose 内使用时 DSN 主机须为 `mysql:3306`，密码须与该 Compose 的 MySQL root 密码一致。

**已按 AutoMigrate 建库的老库：明确为旧库升级未支持。**旧默认 Compose 创建的 `mysql_data` 卷（含 `sql/init.sql` 建表或 `AutoMigrate` 建表）属于未知历史结构，迁移器会拒绝，API 保持停止；不提供自动转换、不回填账本、不把旧库静默标成 v1。开发卷只有一条路：确认其中无须保留的业务数据后，`docker compose down -v` 删卷重建（空库全新安装）。任何含真实业务数据的旧库都保持停止，不得删账本、手工补表或强行启动，须按本文“旧数据库升级”一节先取 schema-only 基线、在隔离副本上设计验证方案（另行立项，不属本次修复）。

**失败与回滚：**迁移失败时 `schema-migrate` 容器以非零退出，backend 因 `service_completed_successfully` 依赖绝不启动；fail-closed 不代表自动回滚，DDL 中途失败可能留下 `applying` 账本或部分对象，此时不要删账本、手工补表或强行启动 API，应先停止操作并从已验证备份恢复，或在隔离副本调查（与生产 Compose 相同）。

## 独立版本化部署入口与支持状态

`docker-compose.managed.yml` 是独立 MySQL 的显式版本化部署入口：它不定义 MySQL 服务、数据卷或 `sql/init.sql`/历史 SQL 挂载，而是使用 `.env.production` 中的 `MYSQL_DSN` 指向已经存在、容器可达的目标数据库。该数据库账号需具备当前迁移所需的 schema DDL 权限。后端设置 `DB_AUTO_MIGRATE=false` 和 `INITIAL_DATA_SEED_ENABLED=false`，并且只有 `/app/schema-migrate` 成功退出后才会启动；默认生产 Compose 也显式关闭这两项。种子关闭阻止启动时更新 `super_admin` 角色、创建管理员或插入默认会员套餐/商家计划。应用配置默认仍开启初始数据种子，因此全新安装仍应使用 `docker-compose.new-install.yml`；managed Compose 虽然接受空 schema，但不会创建初始管理员或默认套餐。

当前只支持两种数据库状态：**空 schema**（目标数据库本身已创建，但没有用户表、视图、例程、事件等对象；迁移器会从 v1 安装至 v17），以及**精确受管的 v1–v16**（账本版本、checksum、迁移状态和 schema 指纹都精确匹配；迁移器只按顺序推进到 v17）。精确受管 v17 会作为已管理状态通过且不再改动。非空旧库、部分初始化/未完成迁移、未知账本、额外对象、checksum 或指纹漂移均不支持，并在 API 启动前 fail-closed；不会将旧库静默标成 v1，也不会回退到 GORM AutoMigrate。

默认生产 Compose 不再有 GORM AutoMigrate 回退，也不因此增加未知旧库升级支持。不要让两套 Compose 同时连接同一数据库；版本化部署前应停掉旧应用写入并完成备份。若迁移在执行 DDL 中途失败，MySQL DDL 可能留下 `applying` 账本或部分对象；fail-closed 不代表自动回滚。此时不要删账本、手工补表或强行启动 API，应先停止操作并从已验证备份恢复，或在隔离副本调查。

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

**可以：**在一次性、隔离、无真实商家/支付数据的空测试 MySQL 上验证版本化空库安装及精确受管 v1–v9 升级；运行默认 Go 测试。生产 Compose 的 Compose 配置和迁移门禁可用离线合成输入验证；MySQL 专用集成测试只能按运维手册所列规则使用临时目录、Unix socket、关闭 TCP 的合成实例。

**不可以：**把 `sql/migrations/*.sql` 或 `migrations/*.sql` 当作可执行升级批次；在生产库或任何既有业务库上试跑/重跑历史迁移；对已有库手动重放 `sql/init.sql`；把 v1 候选基线当作旧版结构或升级方案；或把版本化迁移的合成测试当作旧库升级验收。未知历史库目前不应连接真实生产验证。

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

v1 基线版本仍为 `schema-v1-candidate-20261113`，SHA-256 为 `d4277d6999a8c5fade3edda12e6d7d9bfbf7874c34a0b0406d03573b68609006`。最新安装入口 `/app/schema-migrate` 先在完全空库安装此 v1，再应用 v2 至 v11 十个严格校验的增量；v1–v9 基线和历史 checksum 不作修改。重复调用只有在最新账本与结构指纹全部精确匹配时才返回已管理。旧的 `ApplyCandidate` 仍只校验 v1，不会把 v2 至 v10 或其他结构当作 v1。

增量版本 `schema-v2-subscription-manual-review-20261113` 的 SHA-256 为 `48ef3ec703a314a213107af20ad415e80dc4aa27cdccaf3349cdc9ab53ea5e26`。它只可从准确的托管 v1 账本、校验和与结构指纹升级；升级后再分别验证原 v1 子结构指纹和完整 v2 指纹。v2 增加 `merchant_subscription_manual_reviews` 与 `merchant_subscription_manual_review_audits` 两张专用表、唯一订单键及四条阻止 UPDATE/DELETE 的数据库触发器，不修改 v1 快照或其他表。只有空库的新安装或完全匹配的 v1 可以推进；未知旧库、部分安装、额外对象、checksum/结构漂移或 `applying` 状态继续拒绝且不自动修复。一次中断留下的 v2 `applying` 状态需要从干净隔离库重新开始或恢复备份，不能删除账本后硬重试。

增量版本 `schema-v3-admin-auth-version-20261114` 的 SHA-256 为 `1398762fd14d351eeb61e2b26f98513a18401bbc77e6da677b91e2f5481f23d4`。它只从准确托管的 v2 增加 `admin_users.auth_version BIGINT UNSIGNED NOT NULL DEFAULT 1`，不更改已批准的 v1 基线或 v2 表/触发器；升级后逐一验证 v1、v2 原始子结构指纹（排除新增列）及完整 v3 指纹。管理员密码恢复、状态/角色分配变化和角色权限变化会原子递增此计数器，使旧 token 不会在状态恢复后复活。

增量版本 `schema-v4-otp-safety-20261114` 的 SHA-256 为 `90aba9d894e7e6af5ecc9904745501472ceb1c20f74248fe360649eca371dde0`。它只从精确托管的 v3 增加 nullable `verification_codes.code_hash CHAR(64)`（保存绑定手机号和用途的 HMAC-SHA-256 摘要）及 `failed_attempts INT UNSIGNED NOT NULL DEFAULT 0`。迁移会把旧 `code` 列内容覆盖为 `REDACTED`、将新摘要列设为不可匹配标记并作废所有旧码；升级期间已有验证码必须重新申请。新增列不改变原 `code` 列结构；迁移后核验 v1、v2、v3 的原始结构指纹（排除 v4 新列）及完整 v4 指纹，v1-v3 快照和校验和保持不变。手机号流程限为每手机号 60 秒冷却、滚动 1 小时最多 5 次发送、每个验证码 10 分钟有效且最多 5 次错误尝试；正确验证码一次性消费。短信日志不再输出验证码。

增量版本 `schema-v5-email-otp-safety-20261115` 的 SHA-256 为 `59a48027c17bc375c3c1d15e22c0db966b3454de8980e9416c3c65d1eff69dbc`。它只从精确托管的 v4 增加 nullable `email_verification_codes.code_hash CHAR(64)`（保存绑定邮箱和用途的 HMAC-SHA-256 摘要）及 `failed_attempts INT UNSIGNED NOT NULL DEFAULT 0`。迁移会把旧 `code` 覆盖为 `REDACTED`、写入不可匹配摘要并作废所有旧邮件验证码；升级后必须重新申请。迁移保留原列结构并验证 v1-v4 历史指纹与完整 v5 指纹。邮箱流程按每邮箱限制为 60 秒冷却、滚动 1 小时最多 5 次；验证码有效 10 分钟、最多 5 次错误尝试，成功后一次性消费。当前未接入邮件服务：开发环境用固定测试码 `123456` 模拟且不记录验证码或邮箱日志；生产、staging 等非开发环境在未配置真实邮件服务时拒绝模拟发送。v5 不连接邮件服务，也不会发出真实邮件。

增量版本 `schema-v6-wholesale-retail-20261116` 的 SHA-256 为 `a925b154315d9d34475e08911fcc0abb25b7986a14c3da11d3bc251f9adc65f0`。它只从精确托管的 v5 增加商品可选销售单位、规格、最低购买量和商家自定义数量档位价 JSON 列，不改写既有订单或支付状态；升级时继续验证 v1-v5 历史结构指纹，完整 v6 账本和商品列指纹通过后才认定已管理。档位阈值与价格由商家自行配置，订单金额由服务端根据商品当前配置重算，订单行保留下单时的单位、规格、基础/实际单价及档位快照。

新安装入口 `ApplyLatest` 依序安装 v1→v2→v3→v4→v5→v6→v7→v8→v9→v10→v11；它仅接受准确托管的 v1 至 v11，未知旧库、部分安装或漂移结构仍 fail-closed。

增量版本 `schema-v7-platform-expenses-20261116` 的 SHA-256 为 `33c1ff24ae4a18f13823c28a9fd904d818fe95a57be1cdc6d1320e23add02a9e`。它只从精确托管的 v6 新增空表 `platform_expenses`，不创建或填入任何费用示例。该表记录实际已发生支出，金额保存为整数分、币种固定为 CNY、日期为 `DATE`；分类只允许云服务/主机、消息/API、人员/外包、获客/差旅、设备和其他，memo 必填且最多 120 字，另存创建人和 UUID 幂等键。数据库 CHECK 约束拒绝非正金额、非白名单分类、非 CNY 币种和空备注；唯一幂等键防止重试重复入账，数据库触发器拒绝 UPDATE/DELETE。写入仅允许 `super_admin`，费用行和不含金额/备注的用途审计在同一事务提交。迁移继续验证 v1-v6 原始结构指纹、v7 完整对象目录与校验和；v7 的 `applying` 状态或结构漂移一律 fail-closed，不自动修复。v7 发布时费用只有追加入口，没有编辑、删除或冲正/调整入口；后续冲正范围由 v8 明确。计划/估算费用与实际支出分离且不参与计算；未登记费用、AI provider/token 成本或缺失成本数据均不能视为 0，页面的“订阅净实收减已登记成本”只是部分差额，不是利润。

增量版本 `schema-v8-platform-expense-reversals-20261119` 的 SHA-256 为 `54b9e6454f13a524d816ce96c04fa3cf0a4ef3325c77d1a1845b8c06b431e40f`。它只从精确托管的 v7 新增空表 `platform_expense_reversals`，不创建示例或迁移历史费用。只有 `super_admin` 可为原平台费用追加一笔或多笔部分/全额冲正；金额、原因和冲正发生日期必填，服务在事务中锁定原费用并校验累计金额不超过原金额。外键、非唯一原费用索引、唯一幂等键、正金额约束和数据库 UPDATE/DELETE 触发器保障关联与追加不可变。每条冲正及不含原因、金额的用途审计同事务提交；原费用行和原有审计均不修改。期间报告按原费用发生日列原始费用、按每笔冲正发生日列抵减，因此冲正只在其发生期间体现，不回溯重算原期间；期间费用净额为该期原费用减该期各笔冲正。迁移验证 v1-v7 历史指纹、v8 完整目录与校验和；不精确的 `applying` 状态或结构漂移均 fail-closed。v8 不增加编辑、删除能力，也不改变其他收入、成本或利润口径。

增量版本 `schema-v9-table-ordering-20261124` 的 SHA-256 为 `63a3ea3cba7de9d208fd82d8ccc0fe6957854098aa31a16be768ed35826b1efb`。它只从精确托管的 v8 新增五张空表：商家桌数配置、桌码、共享桌位会话、购物车行和订单桌号关联。桌数和桌号受 0–300／1–300 范围约束；桌位唯一键、外键、状态及数量 CHECK 约束均做精确结构核验。v9 不改写 v1–v8 的表、迁移记录、checksum 或历史指纹；升级只接受完整匹配的 v1–v8 受管结构，重复运行仅接受完整匹配的 v9。

增量版本 `schema-v10-merchant-table-labels-20261124` 的 SHA-256 为 `c9769345b9d994a7a42e856fcb05dc9f56e9c001a1e7fe7cbd9f4d09aa5ab7c9`。它只从精确托管的 v9 追加 `store_tables.table_label`（商家当前展示标签，最多 64 字符）和 nullable `table_order_details.table_label_snapshot`（结算时的标签快照）；桌号与桌码仍是二维码、会话和结算身份。迁移严格校验 v9 原结构及账本，并在 v10 完成后通过排除这两列的 v9 指纹核对历史结构未变，再校验新增列形状与完整 v10 指纹。历史详情行不回填快照，读取时按原桌号显示默认标签；新订单保存结算时标签快照，因此后续改名不会重写历史订单。v10 不触碰订单付款渠道或商家自有收款码配置。

增量版本 `schema-v11-order-items-20261127` 的 SHA-256 为 `d5caa30c03838c6c90b975ef7673a4c67000e2e1c503e3395a51bcb20df5b322`。它只从精确托管的 v10 新增 `order_items` 明细表（归属订单、商品、商品名快照、数量、单价、金额，均以分为单位；数量 1–999、单价与金额不小于 0；外键指向 `orders` 与 `store_products`）并向 `orders` 追加 `channel` 列（`varchar(24)`，默认 `store_qr`，取值 `store_qr` / `table` / `share`）。迁移同时把有 `table_order_details` 记录的历史订单回填为 `table`，其余历史订单保持默认 `store_qr`，不虚构分享归因。`channel` 列没有数据库索引与 CHECK 约束——`orders` 是 v1 表，索引或 CHECK 会改变全部已钉住的历史结构指纹；取值范围由 service 层 `ValidOrderChannel` 校验。迁移严格校验 v10 原结构及账本，并在 v11 完成后通过排除新表与新列的 v9/v10 指纹核对历史结构未变，再校验新表形状与完整 v11 指纹。`order_items` 行的运行时写入是后续工作：建表本身不回填历史订单的 Items JSON 快照。

增量版本 `schema-v16-customer-profiles-20261128` 的 SHA-256 为 `de9c0c0967b94abfd750cd15e681227ac60728486ab1611a305d0387ef116c96`。它只从精确托管的 v15 新增 `customer_profiles` 物化表（每商户每手机号一行，`(merchant_id, customer_phone)` 唯一键），并用历史有效订单回填：只存事实，不存标签——订单数、毛额（不扣退款，与旧画像口径一致）、优惠额、首次/最近付款时间、最近下单时间（`MAX(created_at)`，下单时间不是付款时间）、最新单号与最新单状态、最新单的门店；AITag 与建议仍在读时由事实加当前时间算出，规则未变。回填与旧实时聚合逐行一致，只有一处有意微变：门店名取最新单的门店，旧 `MAX(stores.name)` 在同一顾客跨多门店下单时是非确定性的。运行时由支付回调（正常与重复/迟到异常分支）与商家手动确认在 `PaidAt` 由空变有值时做 O(1) 增量 upsert（次数加一、金额累加、`last_*` 取最新），接单/完成/关单仅当该单为该顾客最新单时更新 `last_status`；未付单、0 元单、空手机号、非到店订单不建行。沉睡分层改读同一表的 `last_paid_at`，与画像的 `last_order_at` 各取所需，不再各扫一次订单表。迁移严格校验 v15 原结构及账本，升级后核对 v1–v15 历史指纹与完整 v16 指纹；`orders.customer_phone` 无索引且 v1 指纹锁定，本次不补索引。

增量版本 `schema-v17-merchant-onboarding-20261128` 的 SHA-256 为 `9df04dcd4f04299595d57d64a610f4b0846d70cbd6a802fb5cb0dba52a752960`。它只从精确托管的 v16 向 `merchants` 追加 `review_reason`（varchar(255)，默认空）、`reviewed_by`（bigint，默认 0）与 `reviewed_at`（datetime(3)，可空）三列，不新增表、索引或 CHECK 约束，取值范围由 service 层校验。历史行不重写：已有 active/suspended 商家保持原状态，三个新列取默认值，拒绝态为 status=pending 且 review_reason 非空、reviewed_at 非空；重新提交清空三列回到干净的待审核。迁移严格校验 v16 原结构及账本，升级后核对 v1–v16 历史指纹与完整 v17 指纹。

历史订阅人工核验是与普通 provider 支付确认分离的流程：后台必须记录管理员对原始凭证及“外部付款确已发生”的明确断言；它把订单标成 `manually_verified`，在专用不可变复核/审计中保存原始条款、证据引用、理由和前后状态，并据所填期限授予订阅权益。它不调用支付渠道、不生成 provider `PaymentRecord`、`PaidAt` 或渠道交易号；普通 provider-confirmed 收入统计只统计原有 `paid` 状态，不把 `manually_verified` 混入。该状态依赖管理员确实核对外部原始证据，系统不会替其验证。

默认 `docker-compose.prod.yml` 和独立 MySQL 的 `docker-compose.managed.yml` 均通过版本化迁移器门禁，只接受空 schema 或精确受管 v1–v11；未知旧库不会启动 API。不要因 Compose 存在或本地合成测试通过就认为未知旧库升级已支持。真实部署和旧数据库升级均须各自验证；本地测试不能替代真实发布演练。MySQL 集成测试只允许使用 `/tmp/cue-schema-migrate-*` 下的临时 Unix socket，且测试数据库须以 `cue_schema_migration_it` 开头；集成用例拒绝 TCP DSN。

### 可重复的本地全流程检查
需要在本地验证批准的 v1 → v2 → v3 → v4 → v5 → v6 → v7 → v8 → v9 → v10 → v11 新安装、精确 v6→v7、v7→v8、v8→v9、v9→v10 与 v10→v11 升级、桌位表约束与标签列、合成订阅人工复核、费用冲正和备份恢复时，运行 `bash scripts/validate-schema-lifecycle.sh`。该脚本使用本次新建的临时 MySQL 8 datadir 和 socket-only 实例，不使用 Docker、`MYSQL_DSN`、`.env` 或生产备份脚本的默认 TCP 目标。完整覆盖与限制见[本地数据库生命周期验证](local-schema-lifecycle-validation.md)；它仍不验证旧库升级或生产备份配置。
