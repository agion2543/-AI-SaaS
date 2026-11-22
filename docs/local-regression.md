# 本地离线回归基线

在 Linux 本地仓库根目录运行：

```bash
bash scripts/test-local-regression.sh
```

这个入口只使用已经存在的本地工具和依赖，不安装或下载任何内容。Go 命令设置 `GOPROXY=off`、`GOSUMDB=off`、`GOTOOLCHAIN=local` 和 `-mod=readonly`；如果需要的模块不在本地缓存中，测试会失败并提示缺少离线依赖，不会联网补齐。前端使用已存在的 `web/node_modules`，不运行 npm/pnpm install。Go 与 Node 测试进程使用净化后的环境，不继承应用数据库凭据或集成测试 DSN。

默认基线覆盖：

- 后端：`go test -count=1 ./...`、`go build ./...` 和 `go vet ./...`。
- 前端：运行 `web/src/` 下全部 `*.test.js` Node 测试，并以本次私有临时目录作为 Vite `outDir` 构建平台端和移动端入口。仓库现有的 `web/dist` 不会被覆盖。
- 合成测试：Bash 语法检查、`test-backup-mysql.sh`、`test-backup-integrity.sh` 和 `test-backup-retention.sh`。它们只使用 mock 或临时合成文件，不连接数据库、不使用真实 age 加密。
- 如果本机已有 `pwsh`，会运行 `test-backup-mysql.ps1` 的 mock 测试；没有 PowerShell 运行时就明确跳过，不下载运行时。

## age/MySQL 恢复演练

恢复演练不是默认流程。只有在非 root 的 Linux 环境中，且调用者明确选择、机器已本地安装 `age`、`age-keygen`、MySQL 8 服务端与客户端、`ss` 时，才用以下命令尝试：

```bash
bash scripts/test-local-regression.sh --with-age-restore
```

缺少任何依赖或 MySQL 版本不符时会安全跳过并说明原因。满足条件后才调用 `test-backup-restore-age.sh`：它生成临时 age 密钥和 synthetic SQL，在私有临时目录启动仅 Unix socket、关闭网络监听的全新 MySQL 8 实例，导入并核对合成记录，然后清理本次文件与数据库。测试脚本本身还会拒绝 root 和带数据库凭据的环境；回归入口向子进程提供净化环境。该测试不会下载工具，也不会接触业务数据库。

当前基线刻意不自动运行其他会启动 MySQL 的集成测试：

- `bash scripts/validate-schema-lifecycle.sh`：独立的 schema 生命周期演练，使用临时 socket-only MySQL；要求非 root 及本机 MySQL 8 等工具。
- `bash scripts/test-customer-order-mysql.sh`：顾客订单 API 合约集成测试，使用它自己的临时 socket-only MySQL；要求非 root 及本机 MySQL 8 等工具。

这些专用脚本会拒绝 `MYSQL_DSN` 等应用凭据，但应只在明确需要验证相应数据库集成时单独运行。Go 默认测试中的数据库集成项因 DSN 未传入而跳过。`test-backup-restore-age.sh` 也只通过上述显式选项调用。

回归入口只清理带本轮专属标记的临时目录；会复用本机 Go 模块/编译缓存和已有前端依赖，不会清理这些缓存、修改生产数据、连接外部服务、写入 GitHub、提交或部署。
