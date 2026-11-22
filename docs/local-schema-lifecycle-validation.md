# 本地数据库生命周期验证

在 Linux 本地用 MySQL 8 验证新安装迁移、人工复核审计、v7 平台费用表、v8 冲正表和合成备份恢复，不需要 Docker：

```bash
bash scripts/validate-schema-lifecycle.sh
```

需要本机已有 `mysqld`、`mysql`、`mysqladmin`、`mysqldump`、`ss` 和 Go；脚本不安装依赖、不联网拉取模块，也不运行前端命令。请以**非 root** 用户执行。schema 测试包括 `internal/schema/v7_platform_expenses_test.go` 和 `internal/schema/v8_platform_expense_reversals_test.go`，路由生命周期测试见 `internal/router/mysql_lifecycle_integration_test.go`。

## 安全边界

脚本为本次运行新建 `/tmp/cue-schema-migrate-*` 随机目录和 MySQL datadir，只使用目录内的 Unix socket；启动参数关闭 TCP、MySQL X Protocol，并用 `@@skip_networking`、`@@datadir`、`@@socket` 和监听端口检查拒绝不符合条件的实例。Go 测试还会再次检查隔离根目录、socket、MySQL 版本和网络状态，数据库名必须使用 `cue_schema_migration_it_` 前缀，并且每次都拒绝重用已存在的测试库。

整个流程不会读取仓库 `.env`、MySQL 全局或用户选项文件、`MYSQL_DSN`、`MYSQL_PWD`，也不连接 `/var/lib/mysql`、TCP 地址、既有业务库、支付渠道或外部账户。任何失败或中断都会尝试停止本次启动的唯一 mysqld 进程；仅当随机目录仍是规范的本次临时目录且数据库进程已停止时，才删除该目录。停止失败时会保留目录以免删除仍在使用的数据。

现有 `scripts/backup-mysql.sh` 默认连接 `127.0.0.1`、默认库名为 `saas_billing`，不适用于这个隔离 socket 演练，所以验证器**不会调用该脚本**。集成测试在显式 socket 上调用本机 `mysqldump` 和 `mysql`，沿用备份脚本的 `--single-transaction --routines --triggers` 方式，把源库恢复到同一全新隔离实例中的第二个全新 schema；不填写或输出任何数据库密码。

## 覆盖什么

- 通过当前 schema 包安装精确的 v1 基线并验证 v1 幂等重跑；在 v1 合成订单及合成旧验证码上运行仓库编译出的真实 `cmd/schema-migrate`，推进 v2 至 v8；同时核验商品配置列、费用与冲正表结构、v1-v8 迁移账本及重复运行返回已管理。
- 单独从精确托管 v6 升级到 v7，再从精确 v7 升级到 v8；确认新增费用和冲正表均无种子行，随后仅在本次隔离库插入 synthetic CNY 费用及多笔关联部分冲正，核对金额与原因、外键、非唯一原费用索引、唯一幂等键、额外对象/不完整账本拒绝升级、结构漂移拒绝重跑，以及 UPDATE/DELETE 触发器阻止篡改费用或冲正行。
- 创建完全虚构的 `manually_verified` 订阅订单、人工复核记录及审计记录；确认未生成 provider `PaymentRecord`、交易号或支付时间。
- 检查迁移账本、表/触发器数量与指纹；执行 dump/restore 后，再用迁移器验证恢复库结构、八个版本校验和及结构指纹一致。
- 在源库和恢复库上验证复核、审计、费用及冲正表的 UPDATE/DELETE 均被数据库触发器拒绝，且原值保留；恢复后的 synthetic expense、reversal 与原账本指纹相符。
- 通过恢复库上的应用只读后台路由读取复核记录，并检查后台仪表盘的 `platform_subscription_revenue` 与 `total_revenue` 都为 0。请求只在进程内经 Gin 测试路由处理，使用合成管理员 token，不会启动对外监听的 API 服务。

## 不能据此推断

恢复目标是同一隔离 mysqld 里的第二个空 schema，不是第二台服务器、不同 MySQL 版本或异地备份介质。这里也没有验证生产脚本的目标主机/账号、密钥、保留和加密策略、权限、存储容量或恢复耗时；没有验证用户提供的历史旧库基线、真实部署配置、真实人工核验证据或外部支付行为。此脚本只验证批准的全新安装 v1 → v2 → v3 → v4 → v5 → v6 → v7 → v8 路径及这组虚构记录在本地 MySQL 8 中可读、可恢复；它不验证未知或非托管历史数据库。迁移器只允许从结构、账本、校验和及指纹均严格匹配的已管理版本升级。
