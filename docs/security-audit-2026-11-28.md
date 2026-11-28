# AI-SaaS 安全审计清单（2026-11-28）

- 分支 `dev/mainline`，HEAD `717b205`，工作区 `~/workspace/ai-saas-dev-work`；审计方式：只读读码 + 基线测试实跑，未改代码、未动生产数据、无真实收款。
- 口径：每项写清现状（有 / 无 / 部分）、文件与函数位置、风险等级；数字注明来源；未验证的写未验证。
- 基线测试（本轮实跑，HEAD `717b205`）：`go build` 通过、`go vet` 通过、`go test ./...` **341 通过 / 27 跳过（MySQL 集成，按设计跳过）/ 0 失败**；JS `node --test src/utils/*.test.js` 0 失败。

## 订阅状态模型（先定结论，后面所有 enforcement 都按它判断）

- 商家订阅**没有独立的“试用”状态**。`model.Merchant.SubscriptionStatus` 的取值只有 `active / inactive / paused / suspended` 等（`internal/model/merchant.go:12`）；`HasActiveSubscriptionAt(now)`（同文件）要求 `SubscriptionStatus == "active"` **且** `SubscriptionExpireAt`（回落 `SubscriptionExpiredAt`）在当前时间之后。
- “试用 7 天”只存在于死代码 `defaultSubscriptionDays("trial")`（`internal/service/admin_service.go:2615`，全仓无调用方）；超管开通只接受 `month / year`（`OpenMerchantSubscription`，同文件约 640 行）；卡密兑换、支付宝回调、人工复核开通走的都是同一条“置 `active` + 写到期时间”的路。因此**试用期商家在系统里就是 `active + 未到期`**，按到期时间拦截不会误拦试用商家；到期、暂停、未开通、被禁用的商家都会被拦。

## 发现汇总

| # | 发现 | 等级 | 状态 |
|---|---|---|---|
| H1 | 公开下单链路不校验订阅：过期/暂停/未开通商家门店仍可接新单 | 高 | 已修复（`1760313`） |
| H2 | `APP_ENV` 未显式设置时开发态能力仍生效（固定验证码 123456、待审核商家自动激活） | 高 | 已修复（`2e21098`） |
| M1 | 商家确认收款无操作人审计留痕；`PaymentRecord` 无操作人字段 | 中 | 已修复审计日志（`6670d37`）；`PaymentRecord` 加操作人列仍遗留（需 v15 迁移） |
| M2 | 退款申请与执行同一人、无二次审批（留痕齐全） | 中 | 遗留（不重建审批流） |
| L1 | 商家侧自有删除/停用操作无审计（删门店、删商品、删促销） | 低 | 已修复（`6670d37`） |
| L2 | 商家 Token 有效期 72 小时偏长 | 低 | 已有缓解，不修 |
| L3 | 默认 Compose 的 MySQL root 密码仍是硬编码 `123456` | 低 | 遗留（生产 `Validate()` 已拦截） |
| L4 | 顾客查单凭可转发的 access token、桌台凭桌码（设计如此） | 低 | 不修，记录在案 |

已核实无问题的项见下文第 2、4、5、6、7 节（鉴权、开发端点、密钥校验、审计覆盖、日志与默认口令）。

## 1. 订阅 enforcement 点

### 1.1 商家端受保护接口：有，服务端强制校验

- `middleware.MerchantSubscriptionMiddleware(db)`（`internal/middleware/auth.go:158`）：每次请求查库取商家，`suspended` 商家拒绝、非 `active` 商家拒绝、`paused` 订阅全锁、其余按 `HasActiveSubscriptionAt` 判断；过期商家仅放行 GET 只读白名单（`expiredMerchantReadOnlyRoutes`：leads、payment-config、stores、orders、refunds、settlements、promotions、group-buy-links 等），写操作一律 403。白名单外的 GET（如 `/ai/insights`、`/coupons`、`/share/config`、`/ops-dashboard/*`）过期即拒绝。
- 挂载位置：legacy `/api/merchant` 与 v1 `/api/v1/merchant` 的 `merchantProtected` 组（`internal/router/router.go:115,322`），覆盖全部付费功能：AI、分享裂变、优惠券、线索、门店/商品、订单/退款/结算、促销、团购、活动目标/复盘、建议执行、手工核销、经营看板。测试覆盖：`internal/middleware/merchant_subscription_test.go`（过期边界、暂停全锁、只读白名单）、`internal/router/merchant_subscription_entitlements_test.go`。
- 未进受保护组的是订阅管理与只读入口，有意为之：`GET /info`、`GET /maintenance-notice`、`GET /announcements`、`GET /maintenance-status`、`GET /check-subscription`、`GET /subscription`、`POST /subscription/orders`、`GET /subscription/orders/:orderNo`、`POST /subscription/orders/:orderNo/paid`、`POST /subscription/cards/redeem`、`PUT /password`。其中 `MarkSubscriptionOrderPaid`（`internal/service/merchant_service.go:376`）只写操作日志、不改订阅状态、不开通——开通必须走支付宝回调验签或超管 `ConfirmMerchantSubscriptionPayment`（人工复核，有审计），商家自助标记不能绕过付费。

### 1.2 H1（高）：公开下单链路不校验订阅——过期商家门店仍可接新单

- `CreateCustomerStoreOrder`（`internal/service/order_service.go:171`）：只校验 `store.Status == "active" && store.IsOpen && store.Merchant.Status == "active"`，**不查 `SubscriptionStatus` / 到期时间**。过期、暂停、未开通订阅的商家，只要门店是 active，顾客扫码仍能下单、付款、商家仍能接单收款。
- 桌台结算 `TableOrderingService.Checkout`（`internal/service/table_ordering.go:389`）与 `createTableOrderInTransaction`：同样只校验门店 active + 商家 Status active + 维护拦截，不校验订阅；已锁定的在途结算单分支直接返回已有订单（在途，不应拦）。
- 追加菜品 `appendCustomerStoreOrderItemsLocked`（`internal/service/order_service.go:466`）：同样只校验门店状态，不校验订阅。
- 留言线索 `MerchantService.CreateCustomerLead`（`internal/service/merchant_service.go:1134`）：同样只校验门店 active，不校验订阅；过期商家仍能持续收新线索（付费 CRM 价值外泄）。
- 不应拦的在途环节（维持现状）：顾客标记已付、支付宝回调、商家确认收款、退款——订单已生成，在途完成不受订阅影响，与维护拦截“只拦新下单”的设计一致。
- 前端有过期守卫（`web/src/router/index.js`：`allowExpiredSubscription / allowExpiredReadOnly` + `merchantSubscriptionAccessState`），但那只是前端跳转，后端公开链路无校验即是绕过口：直接调接口即可绕过前端。

### 1.3 其它订阅相关校验：有

- `CreateSubscriptionOrder`（`internal/service/merchant_service.go:445`）：禁用的商家拒绝下单；套餐快照（名称/价格/天数）服务端取自 `MerchantPlan`，不信任客户端传价；`Items: "[]"`（MySQL JSON 列问题已修）。
- `RedeemSubscriptionCard`：卡密兑换按卡面时长续期，未到期时从现有到期时间叠加（同文件约 248–290 行）。
- 支付宝回调激活：仅验签成功且订单 `pending` 时激活（`internal/service/order_service.go:818` 起），过期订单回调走 `paid_review_required` 人工复核，不自动履约/开通（已有测试覆盖）。

## 2. 鉴权

### 2.1 Token 校验与已停用账号：有

- `middleware.Auth(cfg, role, db)`（`internal/middleware/auth.go:31`）：验 Bearer JWT 签名与过期；`admin` 角色每次请求查库（状态须 `active`、角色须存在、claim 角色与库内角色一致、凭据版本比对）；`merchant / user` 角色每次请求查 `model.User`（状态须 `active`，凭据版本比对）。
- 凭据版本绑定（`internal/utils/jwt.go:29,53`）：`AdminCredentialVersion` 覆盖 `AuthVersion + PasswordHash + Status + RoleID + Role.Code + Permissions`；`UserCredentialVersion` 覆盖 `UserID + Email + Phone + PasswordHash + Status + Role + MerchantID`。改密、改权、禁用、换商家归属都会使已发 Token 立即失效——**已停用账号的 Token 不再有效**，不是只靠过期时间。
- Token 有效期：商家/用户 72 小时、超管 24 小时（`internal/service/auth_service.go:85,103,126,163`、`internal/service/merchant_service.go:148,183`）。72 小时偏长，但凭据版本已提供即时吊销能力，评 L2，不修。

### 2.2 角色区分：有

- Auth 按 `requiredRole` 比对 claim 角色；超管侧 `AdminAuthorization`（`internal/middleware/admin_permissions.go`）按方法 + 路径查显式映射，未映射直接 403（fail-closed），普通 admin 与 super_admin 区分；财务接口用独立权限键 `platform_finance`（`internal/adminaccess/permissions.go:14,134`），仅超管持有。覆盖测试 `TestEveryRegisteredAdminRouteHasExplicitAuthorizationMapping`（`internal/router/admin_permission_coverage_test.go`）本轮基线通过。
- 套餐调整 `SaveMerchantPlan`：价格 <0、周期 ≤0 服务端拒绝，改动记审计（`internal/service/admin_service.go:468`）。

### 2.3 merchant_id 冒用：无反例

- 商家侧全部接口的 `merchant_id` 取自 Token 上下文（`currentMerchantID` / `currentMerchantOrderMerchantID`，`internal/controller/merchant_controller.go:991`、`internal/controller/order_controller.go:1155`），不读请求体/Query 里的 `merchant_id`。
- 服务层按 `merchant_id` 过滤：订单 `FindByMerchantAndID`、商品先验门店归属、优惠券、AI 用量、结算、线索全部带商家条件；跨商家访问返回“不存在”（不泄露存在性）。抽查无反例。
- Token 里的 `merchant_id` 本身不可篡改：JWT 签名 + 凭据版本摘要含 `MerchantID`，改 claim 即验签/版本双双失败。`GenerateToken`（无凭据版本）全仓无生产调用方，只有 `GenerateUserToken / GenerateAdminToken` 在用。
- 超管侧接口的 `merchant_id` 是路径/Query 参数（`GET /admin/merchants/:id/...` 等），属于超管按权限操作指定商家，设计如此；`parseMerchantIDQuery` 要求必填且为合法数字。
- 边界（L4，设计如此，不修）：顾客查单凭订单号 + `X-Order-Access-Token`、桌台操作凭桌码——token/桌码泄露即对应订单/桌位可见；分享扫码、团购跳转、领券（限流 + 手机号）等公开端点无登录，属产品设计。

## 3. 支付手工确认与退款

### 3.1 手工确认收款：部分（金额取服务端、但无操作人/凭证）

- 顾客标记已付 `MarkCustomerStoreOrderPaid`（`internal/service/order_service.go:429`）：只把状态改为 `payment_confirming`，不设 `PaidAt`、不写 `PaymentRecord`；仅写一条操作日志（动作/文本/时间），无操作人、无金额、无付款凭证——顾客身份只凭订单 access token，本环节本就不产生可信付款证据，留痕弱是模型固有限制。
- 商家确认 `ConfirmMerchantOrderPayment`（同文件 1138 行）：金额取服务端 `order.TotalAmount`（客户端改不了金额）；生成 `PaymentRecord`（渠道 `merchant_qr`，`TransactionNo = manual-<orderNo>-<unix>` 合成值，`RawPayload` 为固定字符串）；**M1（中）**：该接口**不记审计日志**（controller 不传 `AuditEvent`，service 不写 `audit_logs`），`PaymentRecord` 模型（`internal/model/order.go`）**无 `OperatorID / OperatorRole` 字段**，事后只能从订单操作日志的文本反推“商家已确认”，无法定位是哪个商家账号点的确认。
- 修复方向：controller 传审计事件、service 在同一事务内写审计行（动作 `merchant_order_payment_confirm`，含订单号、金额、渠道、合成交易号、操作人）；`PaymentRecord` 加操作人列需新迁移（v15），本轮不做，列遗留。

### 3.2 退款：审核与执行不分离（M2，中，遗留），但留痕与防重放齐全

- `processReservedRefund`（`internal/service/order_refund.go:32`）：行锁 + 幂等键 + 预占额度（`processing` 预占，`success/unknown` 互斥），超额/重复/未知态全部拒绝；非支付宝渠道（手工收款码订单）直接拒绝——“无验证过的退款渠道时不得记成功”，与 P0 报告的基线口径一致，main 版“记 success”的旧口径已不存在。
- 审计：商家/超管退款均传 `AuditEvent`（`internal/controller/order_controller.go:465,491`），含金额、是否全额、原因、退款单号、阶段 `provider_dispatch_intent`；`RefundRecord` 有 `OperatorID / OperatorRole`；未知结果进对账（`refund_reconciliation` + append-only 审计 + 人工复核/解决接口）。
- **不分离**：同一商家（或同一超管）发起即直接调支付宝执行退款，无第二人审批、无金额阈值分级。缓解是全程留痕 + 幂等 + 对账。引入审批流是产品级改动（影响商家售后时效），本轮不重建，列遗留；建议后续按金额阈值设超管复核。

## 4. 开发端点与 APP_ENV

### 4.1 开发重置端点：有，三重门控

- 路由注册需 `cfg.DevPasswordResetEnabled && AppEnv == "development"`（`internal/router/router.go:289`）；服务层 `AuthService.DevResetPassword` 再验一次（`internal/service/auth_service.go:393`）；`DevPasswordResetEnabled` 的定义是 **显式** `APP_ENV=development`（`internal/config/config.go:52`，直接读 `os.Getenv("APP_ENV")`，未设置时为 false）。改密后 `auth_version + 1` 使旧 Token 失效（有测试覆盖）。**该端点在 `APP_ENV` 未设置时不开放**，已兜底。

### 4.2 H2（高）：`APP_ENV` 未设置时，固定验证码与自动激活仍按开发态生效

- 默认 `APP_ENV` 是 `development`（`internal/config/config.go:52`，`getEnv("APP_ENV", "development")`）。`Validate()` 只在 `APP_ENV == "production"` 时才校验 JWT/AES/DSN 默认值；忘设 `APP_ENV=production` 的部署会带着默认密钥启动——这是 P0 已知的残留风险，本次核实到两条**同根的具体生效路径**：
  1. 固定验证码：`generateMockSMSCode`（`internal/service/sms_policy.go:38`）与 `generateMockEmailCode`（`internal/service/email_otp_policy.go:28`）只看**默认化后**的 `AppEnv == "development"`，`APP_ENV` 未设置时同样返回固定码 `123456`。该码可用于商家注册、登录、密码重置、邮箱绑定——等于部署忘设环境变量时，任意知道手机号的人都能用 `123456` 走通验证码流程。
  2. 待审核商家自动激活：`seedInitialData`（`internal/bootstrap/app.go:174`）以 `cfg.AppEnv != "production"` 为条件，把全部 `pending` 商家置为 `active`，并播演示团购链接；`APP_ENV` 未设置时同样生效。
- 生产预检脚本（`scripts/preflight-production-deploy.py:149`）要求 `APP_ENV=production`，但那是部署前人工跑的脚本，不是启动时兜底；超管“安全检查”页只给 warning，不拦截。
- 修复方向：危险的开发态能力一律以**显式** `APP_ENV=development`（即现有 `DevPasswordResetEnabled` 语义）为准，不以默认化后的 `AppEnv` 为准；`AppEnv` 默认值保持 `development` 不变（现有 `config_test.go` 已钉住该默认值），只改能力门控。

## 5. 生产密钥校验

- 有（`internal/config/config.go:Validate`）：生产环境拒绝 JWT <32 位 / 占位符 / 默认值、AES 占位符或默认值、DSN 为空 / 含 `root:123456@` / 占位符；`AES_SECRET` 长度必须恰好 32 字节（**所有环境**都校验）；`INITIAL_ADMIN_PASSWORD` 非空时必须 ≥12 位且非占位符（**所有环境**都校验）；`TRUSTED_PROXIES` 拒绝通配 CIDR 与非法值；OTP 验证码哈希要求 JWT ≥32 字节（`internal/service/sms_policy.go`：`errOTPHashKeyTooShort`）。
- 无硬编码默认超管密码：无密码时 `seedAdminUser` 拒绝建账号（`internal/bootstrap/app.go`）；默认 Compose 的 MySQL root 密码 `123456` 是 L3 遗留（本地开发用，生产 `Validate()` 会因 DSN 含它而拒绝启动）。
- 唯一缺口即 H2：校验只在显式 production 时触发，忘设 `APP_ENV` 时默认密钥在“development”名义下生效。

## 6. 审计日志

- 有，且覆盖面广：套餐调整（`merchant_plan_create/update`，含改前改后快照）、订阅开通/暂停/恢复/停止（`merchant_subscription_*`，含改前改后状态）、退款（商家/超管，含金额与阶段）、权限/角色/管理员增改（`admin_access_*`）、账号停用与手机号修改、会员手动开通、系统配置更新（加密项不记明文）、AI 价格/账单/预算/紧急开关、公告/维护计划/活动目标/复盘、平台费用与冲销、收入台账、敏感读（顾客线索列表、订单详情、顾客联系方式查看、订单导出）——均经 `AuditEvent / AppendAuditTx` 在同一事务内落库，失败则业务操作一起回滚（有 `audit_atomicity_test.go` 覆盖）。
- 缺口：M1（商家确认收款）、L1（商家侧删门店/删商品/删促销——`DisableStore / DisableStoreProduct / DeletePromotion` 无审计事件，只有状态变更本身）。

## 7. 其它发现

- **前端藏按钮但后端无校验：未发现。** 商家前端过期守卫（`web/src/router/index.js`）只是体验层；后端 `MerchantSubscriptionMiddleware` 与 `AdminAuthorization` 是最终关口；超管路由覆盖测试强制每个已注册超管路由都有显式授权映射，未映射即 403。过期只读白名单前后端一致（前端 `allowExpiredReadOnly` 对应后端 `expiredMerchantReadOnlyRoutes`；P0 提到的 3 处不一致已在 P2/P3 对齐）。
- **日志泄露敏感数据：未发现。** 请求日志中间件（`internal/middleware/logger.go`）只记 Gin 路由模板、状态码、耗时、IP，不记原始 URL、Query、请求体；全仓 `log.Printf` 只有 2 处（订单过期 worker 的计数）；审计明细有去敏测试（`internal/service/admin_access_audit_test.go`：明细不得含密码/账号身份）。
- **默认口令或测试账号残留：未发现。** 无写死密码、无测试账号种子；固定验证码 `123456` 只在开发态模拟发送路径存在（见 H2），前端多处文案写明“开发环境可用 123456”（`MerchantRegister.vue`、`MerchantLoginView.vue` 等），生产忘设环境变量时即 H2。
- **CORS**：`Access-Control-Allow-Origin` 只取配置的 `FrontendURL`，不反射请求 Origin；`Allow-Credentials: true`；方法/头白名单固定。无通配。
- **上传文件**：`engine.Static("/uploads", ...)` 公开服务上传目录，属设计（商品图、收款码需顾客端展示）；收款码文件名是 UUID（`internal/controller/merchant_payment_qr.go:278`），不可枚举；商家收款码上传需先经登录 + 订阅保护，定稿需显式保存支付配置。
- **死代码**：`defaultSubscriptionDays`（`trial` 分支）无调用方；`GenerateToken`（无凭据版本）无生产调用方。均不构成风险，记录在案，不删。
- **legacy `/api` 路由**：与 v1 同一套中间件（Auth + 订阅中间件 + 超管授权），功能为 v1 子集，冻结不再扩展；无额外缺口。
- **订阅订单金额校验**：`merchantSubscriptionTermsFromOrder`（`internal/service/order_service.go:762`）校验订单快照与金额一致，防止改价后按快照开通；`ConfirmMerchantSubscriptionPayment` 与回调共用该校验。

## 修复记录（2026-11-28，dev/mainline，未推送）

- H1（`1760313`）：新增 `internal/service/merchant_subscription_gate.go`（`merchantAcceptsNewStoreOrders`：商家 active 且订阅 active、到期时间在未来）；`CreateCustomerStoreOrder`、桌位结算建单、追加菜品、创建线索四处共用该门控；在途付款/确认/退款不动；试用期商家（active + 未到期）正常通过。测试：新增 `merchant_subscription_gate_test.go`（下单 7 例、桌位结算、线索）；共享夹具商家改为已订阅（`order_boundary_test.go`、`order_product_pricing_test.go`、`table_ordering_controller_test.go`、MySQL 顾客下单夹具）。
- H2（`2e21098`）：固定短信/邮箱验证码（`generateMockSMSCode / generateMockEmailCode`）与种子自动激活改以显式 `APP_ENV=development`（`DevPasswordResetEnabled`）为准；`AppEnv` 默认值保持 `development` 不变。测试：隐式开发默认值下固定码被拒、种子不自动激活；OTP 策略测试改为显式设置该标志。
- M1 + L1（`6670d37`）：`ConfirmMerchantOrderPayment` 接审计事件、同事务写审计行（动作 `merchant_order_payment_confirm`，含操作人/订单/商家）；`DisableStore / DisableStoreProduct / DeletePromotion` 同样补审计（`merchant_store_disable / merchant_product_disable / merchant_promotion_delete`）；`auditPurposeForAction` 补 4 条中文事由。测试：新增 `merchant_payment_confirm_audit_test.go`（2 测试）。
- 会员订单 `Items` 空值问题：审计时核实已在 `350d2d9` 修过（`Items: "[]"`），本次未重复改动。
- 未修（遗留）：M2 退款审批分离、L2 Token 72 小时有效期、L3 默认 Compose MySQL root 密码、L4 access-token/桌码模型、`PaymentRecord` 操作人列（需 v15 迁移）。

## 修复计划（任务二对应关系）

| 修复 | 对应发现 | 做法 |
|---|---|---|
| 公开下单链路加订阅门 | H1 | `CreateCustomerStoreOrder`、桌台结算新单分支、追加菜品、创建线索四处共用同一门控：商家 `Status == active` 且 `HasActiveSubscriptionAt(now)`，否则拒绝新单；在途订单/支付/退款不动 |
| 开发态能力改显式门控 | H2 | 固定短信/邮箱验证码与种子自动激活改以显式 `APP_ENV=development`（`DevPasswordResetEnabled` 语义）为准 |
| 商家确认收款补审计 | M1 | controller 传审计事件、service 同事务写审计行 |
| 商家侧删除补审计 | L1 | `DisableStore / DisableStoreProduct / DeletePromotion` 补审计事件 |
| 不修（遗留） | M2、L2、L3、L4 | 退款审批分离、Token 有效期、默认 Compose 密码、access-token/桌码模型 |
