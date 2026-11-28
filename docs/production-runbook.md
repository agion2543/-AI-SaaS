# 生产上线与日常巡检手册

这份手册用于个人开发阶段的低成本上线。目标不是一次性做成大型企业发布体系，而是保证每次上线都能回答四个问题：现在运行是否安全、数据库是否可恢复、发布失败怎么退回、每天应该先看哪里。

## 1. 当前推荐上线形态

- 平台后台、商家端、顾客端共用一套后端 API。
- 前端使用 Vue 构建后由 Nginx 或对象存储/CDN 托管。
- 后端使用 Go 二进制或 Docker Compose 部署。
- MySQL 使用独立数据库账号，不使用 root 作为业务连接账号。
- 平台收 SaaS 订阅费，顾客点单款优先进入商家自有收款账户，平台只做订单、退款、对账和运营巡检。

## 2. 首次部署顺序

1. 准备服务器、域名、HTTPS 和 MySQL。
2. 复制 `.env.production.example` 为生产环境变量文件。
3. 修改 `APP_ENV=production`、`APP_URL`、`FRONTEND_URL`、`JWT_SECRET`、`AES_SECRET`、`INITIAL_ADMIN_PASSWORD`、`MYSQL_DSN`；在安全环境中设置非空 `MYSQL_ROOT_PASSWORD` 和 `MYSQL_APP_PASSWORD`，且后者须与 `MYSQL_DSN` 密码一致。
4. 全新数据库安装使用 `docker compose -p saas-fresh-v1 -f docker-compose.new-install.yml up -d --build`；该入口专用新数据卷，禁止用于已有数据库。
5. 访问 `/health` 确认 HTTP 进程有响应，再访问 `/ready` 确认数据库连接可用；`/health` 不代表数据库已就绪。
6. 登录平台后台，立即修改初始化管理员密码。
7. 打开“运营健康中心”，优先处理危险项。
8. 用测试商家跑通：商家入驻、商品上架、顾客下单、支付状态页、商家接单、小票打印、退款记录。

`AES_SECRET` 必须恰好为 32 字节，程序将其 UTF-8 字节原样用作 AES-256 密钥；不要填入 64 个十六进制字符并期待程序解码。该密钥必须跨重启和升级保持不变：系统配置密文依赖当前密钥，直接更换会使已有密文无法读取。当前项目没有密钥环或自动轮换流程；轮换前必须另行设计并在隔离副本验证“旧密钥解密、用新密钥重加密、核验后切换”的迁移。密钥不匹配时，后台加密配置读取会失败并返回错误，不会把无法解密的密文当明文返回。

`OTP_SEND_RATE_LIMIT_PER_IP_PER_MINUTE` 控制所有 `POST .../send-code` 路由共享的单 IP 发码上限，默认每分钟 10 次；它与手机号/邮箱自身的冷却和滚动小时限额叠加。该计数器是进程内固定窗口：同一实例上的各发码路由共享，多个 API 副本之间不共享；多副本部署不能把它视作全局 IP 配额，且客户端 IP 识别须与受信任代理配置一致。

`TRUSTED_PROXIES` 默认为空，应用会忽略未经信任的 `X-Forwarded-For` / `X-Real-IP`，并以 TCP 对端地址作为客户端 IP；直接访问时这最安全。若 API 位于反向代理之后，未配置时限流和审计会看到代理地址，多个客户可能共享一个 IP 桶。只有在代理会正确维护转发头、且后端网络禁止绕过代理直连时，才将 `TRUSTED_PROXIES` 设置为该部署实际使用的代理 IP/CIDR 逗号列表；不存在可通用复制的网段，不要设置 `0.0.0.0/0` 或 `::/0`。多级代理应按实际链路配置可信代理范围。

## 3. 每次升级发布前

1. 记录当前版本或提交号。
2. 配置 `AGE_RECIPIENTS_FILE` 公钥清单及 `age`，再运行 Bash 或 Windows PowerShell 备份脚本；确认生成非空 `.sql.age`。数据库密码只通过受控环境变量提供，不要放在命令参数中；对应私钥 identity 必须另行安全保管并留有离线副本。
3. 将 `scripts/backup-integrity.py` 的 SHA-256 清单保存到独立目录，创建后再运行 `verify`。清单可校验密文变化，但不是加密或签名，也不证明备份可恢复。
4. 运行 `scripts/backup-retention.py --backup-dir <明确目录>` 预览 30 天以上的过期密文候选；只有逐项核对并确认后，才使用该预览令牌运行 `--apply --confirm-token`。工具不自动运行、不删明文 `.sql`，且会保留最后一份密文。
5. 明确本次是空库首次安装还是既有数据库升级；旧库升级前先满足本节的 schema 基线要求，不按文件名猜测 SQL 顺序。
6. 本地执行 `go build -o saas-server.exe ./cmd/server`。
7. 本地执行 `cd web && npm.cmd run build`。
8. 确认 `.env.production`、私钥、数据库密码没有进入 GitHub。
9. 在业务低峰期发布。

如需本地复跑 Bash 加密备份的合成恢复演练，在 Linux 非 root 环境执行 `bash scripts/test-backup-restore-age.sh`。它要求本地已有 `age`、`age-keygen`、MySQL 8.x（`mysqld`、`mysql`、`mysqladmin`）、`ss`、`ps` 及常见 Linux 命令行工具，不会联网、下载或安装工具；流程使用 mock `mysqldump`、合成 SQL、临时 age 密钥和仅 Unix socket 的临时 MySQL。该测试不读取现有/生产备份，也不证明生产密钥、异地副本或 Windows PowerShell 恢复可用，不能替代生产恢复验证。

## 4. 数据库迁移规则

`docker-compose.prod.yml` 保留现有 `mysql_prod_data` 卷键，但命名卷带项目名前缀；切换时沿用旧部署相同的 `-p` 项目名，若旧命令未指定 `-p`，则从同一项目目录执行。不再挂载 `sql/init.sql` 或历史 SQL；Compose 先等待 MySQL healthy，再运行 `/app/schema-migrate`，只有迁移成功后才启动 API，并显式关闭 `DB_AUTO_MIGRATE` 与 `INITIAL_DATA_SEED_ENABLED`。运行前确认 `.env.production` 的 `MYSQL_DSN` 指向此 Compose 的 `mysql:3306/saas_billing`，密码与 `MYSQL_APP_PASSWORD` 相同；生产与全新安装 Compose 都要求 `MYSQL_ROOT_PASSWORD`、`MYSQL_APP_PASSWORD` 非空，否则拒绝启动。独立 MySQL 使用 `docker-compose.managed.yml`。迁移器只接受真正空 schema（安装 v1→v11）、精确受管 v1–v10（升级至 v11）或精确受管 v11（no-op）；未知历史库、部分初始化或结构漂移会拒绝，backend 不会启动，也不会回退到 GORM `AutoMigrate`。全新安装请使用 `docker-compose.new-install.yml`，不要同时运行两条路径：它使用独立专用卷，从版本化 v1 基线安装，并保留初始管理员和默认套餐种子。v1 候选来源为 HEAD `15274c8` 加 2026-11-13 生成时的本地未提交模型改动，仅批准用于全新安装，不适用于历史库。

若数据库已独立创建，可选择 `docker-compose.managed.yml` 版本化入口：它不带 MySQL 服务/数据卷，按 `.env.production` 的 `MYSQL_DSN` 连接目标库，并且只在迁移成功后启动 API。后端设置 `DB_AUTO_MIGRATE=false` 和 `INITIAL_DATA_SEED_ENABLED=false`，所以升级启动不会执行初始角色、管理员、会员套餐或商家计划种子写入；生产 Compose 和 managed Compose 都显式关闭这类启动写入。虽然该迁移器接受空 schema，但这条路径不初始化应用种子；真正新装且需要初始管理员、默认套餐/计划时，仍用上面的 `docker-compose.new-install.yml`。支持目标仅为真正空 schema（安装 v1→v11）或 checksum、账本及 schema 指纹精确匹配的受管 v1–v10（升级至 v11）；精确 v11 为 no-op。所有未知旧结构、部分迁移、额外对象或结构漂移均拒绝。切换前先备份、停止旧应用写入，并确认 DSN 指向预期数据库；不要让两套 Compose 同时连接同一库。

历史 SQL 分散在 `sql/migrations/` 和 `migrations/`，当前没有可信旧版 schema 基线或迁移版本表，按文件名顺序并不可靠。**不要**把任一目录或两个目录当作可直接批量执行的升级清单，也不要对生产库/既有业务库手动重放 `sql/init.sql`。旧库升级目前未支持、未验证；先取得对应旧版本的 schema-only 基线，再在隔离副本上设计并验证变更。详细依赖、重跑与数据副作用风险见[数据库迁移现状与安全边界](database-migration-status.md)。

迁移容器成功退出才会启动 API；迁移器拒绝未知非空库、旧版结构、部分初始化和任何版本/校验和/结构指纹不匹配，不会静默把旧库标记为 v1。生产 Compose 不再运行应用侧 GORM AutoMigrate；目标库若未精确受管，就必须保持停止并先完成基线识别和隔离副本验证。迁移 DDL 中途失败可能留下 `applying` 账本或部分对象；不要删账本或强行启动 API，应停止并从经验证备份恢复，或只在隔离副本排查。具体账本行为、来源和安全限制见[数据库迁移现状与安全边界](database-migration-status.md)。不要按历史 SQL 文件名猜测执行顺序。

### MySQL 专用集成测试

默认 `go test ./...` 不运行这些测试。它们只可用于 MySQL 8.0.46、TCP 已关闭、位于
`/tmp/cue-refund-mysql-*` 下的临时 Unix socket，以及名称以 `cue_refund_p1_`
开头的空合成数据库上运行：

```bash
MYSQL_REFUND_IT_DSN='root@unix(/tmp/cue-refund-mysql-<id>/mysql.sock)/cue_refund_p1_<id>?parseTime=true&loc=UTC' \
  go test ./internal/service -run '^TestMySQL' -count=1 -v
```

该测试会为迁移顺序用例新建并清理一个同前缀临时数据库；不得指向业务库或外部主机。

本地可用 `bash scripts/test-otp-mysql-concurrency.sh` 验证 OTP 首次发码与消费并发。脚本要求非 root 环境和本机 MySQL 8.x；会新建仅 Unix socket、禁用 TCP 的临时 MySQL，确认默认隔离级别为 `REPEATABLE-READ`，使用合成手机号/邮箱/验证码并清理本次临时目录。它不连接业务库，也不能证明多副本限流或生产数据库配置。

### 退款预留逾期与人工核对

平台后台“退款异常核对”同时显示 `processing` 与 `unknown` 预留。`processing`
距持久化的 `updated_at` 已满 **15 分钟**时只作逾期提示（历史时间为空时回退至创建时间）；
这不是渠道退款 SLA，也不代表成功、失败或可释放额度。当前仓库未定义退款渠道的更长处理 SLO；
支付下单的 30 分钟有效期不是退款处理时限。正式接入渠道前须结合实际退款延迟复核此人工分流阈值。

只有平台 `admin` / `super_admin` 可填写必填操作原因，将逾期 `processing` 转为 `unknown`。
服务器在事务内重新检查时间与状态，按订单、退款的顺序加锁，并在同一事务追加操作员、时间、
前后状态、金额及原因的不可修改审计。相同操作重试不会重复转态或追加审计；未满 15 分钟的请求不可强转。

该操作不释放预留额度、不计入已退款金额、不重试或查询支付渠道，也不自动决定退款结果。
`unknown` 仍须由平台管理员在独立渠道后台核验后，填写渠道侧退款/查询凭证号与核对说明，
才能记录确定成功或确定失败。已有后台处理的迟到结果不得覆盖 `unknown`；没有外部证据时继续保留额度。
商户与顾客不能执行上述转态或人工核对。请每日检查逾期项，不要以换幂等键或直接改库替代核对。

## 5. 回滚流程

如果发布后出现严重问题，按这个顺序处理：

1. 先判断是否影响收款、下单、商家接单或后台登录。
2. 如果只是前端展示问题，优先回退前端静态包。
3. 如果后端接口异常，停止新版本后端，恢复上一版二进制或镜像。
4. 如果数据库迁移已改变结构，先评估是否可兼容旧版本；不可兼容时从上线前备份恢复。
5. 记录事故原因、影响订单、处理时间和后续修复项。

## 6. 每日运营巡检

每天打开平台后台后，先看“运营健康中心”：

- 交易链路：待支付、支付失败、待接单、超时履约。
- 支付退款：退款记录、退款原因缺失、沙箱支付配置。
- 结算对账：待结算、逾期结算、未配置收款资料。
- 商家风险：冻结商家、待审核商家、未关闭跟进。
- 商品准备：可营业门店、上架商品、无图商品。
- 部署配置：生产环境、域名、密钥、备份、限流、日志。

## 7. MVP 上线前最低通过标准

- 管理员默认密码已经修改。
- `APP_ENV=production`。
- `JWT_SECRET` 和 `AES_SECRET` 已替换为强随机值。
- MySQL 不暴露公网，业务连接不使用 root。
- 至少有一套已验证可恢复的数据库备份；本地合成演练不满足此项。
- 至少一个真实商家可完成商品上架和接单。
- 顾客端能完成扫码浏览、下单、支付状态查看和失败重试。
- 小票包含订单号、取餐码、商品、客户备注、商家备注和金额。
- 运营健康中心没有危险项，或危险项已有明确处理计划。

## 8. 后续增强方向

- 接入迁移版本表，避免人工判断 SQL 是否执行。
- 接入日志轮转和错误告警。
- 增加订单支付失败率、退款率、商家响应超时的阈值告警。
- 增加数据库备份恢复演练记录。
- 真实支付资质准备好后，再接入服务商分账或子商户模式。

## 9. 时区约定（Asia/Shanghai）

全平台按北京时间计日、计月，约定见 `docs/production-deploy.md` 第 10 节。日常巡检与每次发布前确认三点：

- 容器 `TZ=Asia/Shanghai` 未被改动，容器内 `date` 显示 `CST`/`+0800`；
- MySQL `SELECT @@global.time_zone, @@system_time_zone;` 为 `SYSTEM` 加 `CST` 或 `Asia/Shanghai`；
- `.env.production` 的 `MYSQL_DSN` 用 `loc=Asia/Shanghai`，不是 `loc=Local`。

不要只改容器时区就以为换了时区：每日总结、复盘、营业额按日归属用代码内固定的北京时间，成本中心等处的 `time.Local` 仍依赖容器 TZ；改时区约定前先改代码。
