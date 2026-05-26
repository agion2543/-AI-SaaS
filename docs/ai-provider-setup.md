# AI 模型接入说明

本项目的 AI 能力采用 OpenAI-compatible `/chat/completions` 协议，优先适配国内可访问模型服务，避免主链路依赖境外网络。

## 推荐接入顺序

1. 开发和演示阶段：`AI_ENABLED=false`，系统使用内置模板兜底，不消耗模型费用。
2. 小规模商家测试：接入 DeepSeek 或阿里云百炼通义千问，先开放 AI 经营建议和裂变海报文案。
3. 商用阶段：增加调用次数限制、缓存、套餐权限和生成记录，避免无控制调用。

## 当前技术口径

- 后端统一调用 `AI_BASE_URL + /chat/completions`。
- 请求头统一使用 `Authorization: Bearer ${AI_API_KEY}`。
- 当前适合优先接入兼容 OpenAI 协议的国内模型；不兼容该协议的供应商后续再新增独立 adapter。
- 没有开启或配置不完整时，系统会返回本地模板兜底，保证商家端功能不空白。

## 国内供应商配置参考

| 供应商 | AI_PROVIDER | AI_BASE_URL | 推荐模型 | 适合阶段 |
| --- | --- | --- | --- | --- |
| DeepSeek | `deepseek` | `https://api.deepseek.com/v1` | `deepseek-chat` | MVP、小规模试运营 |
| 阿里云百炼通义 | `qwen` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` | 稳定商用、国内云资源整合 |
| 智谱开放平台 | `zhipu` | 以官方兼容模式地址为准 | 以官方兼容模型名为准 | 备选供应商 |
| Moonshot/Kimi | `moonshot` | 以官方兼容模式地址为准 | 以官方兼容模型名为准 | 长文本营销素材备选 |

> 注意：不同供应商的 base url、模型名和额度政策会变动，正式接入前以供应商控制台最新说明为准。

## DeepSeek 示例

```env
AI_ENABLED=true
AI_PROVIDER=deepseek
AI_BASE_URL=https://api.deepseek.com/v1
AI_API_KEY=sk-xxxx
AI_MODEL=deepseek-chat
AI_TIMEOUT_SECONDS=20
```

## 阿里云百炼通义千问示例

```env
AI_ENABLED=true
AI_PROVIDER=qwen
AI_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
AI_API_KEY=sk-xxxx
AI_MODEL=qwen-plus
AI_TIMEOUT_SECONDS=20
```

## 智谱 / Kimi 等后续供应商

如果供应商提供 OpenAI-compatible `/chat/completions`：

```env
AI_ENABLED=true
AI_PROVIDER=provider_name
AI_BASE_URL=https://provider-compatible-base-url/v1
AI_API_KEY=replace-with-provider-api-key
AI_MODEL=provider-model-name
AI_TIMEOUT_SECONDS=20
```

如果供应商不兼容 OpenAI 协议，建议后续在 `internal/service/ai_service.go` 里按 `AI_PROVIDER` 增加 adapter，不要把不同供应商的特殊参数散落到业务代码里。

## 当前已接入的业务场景

- 商家端 `AI 经营增长中心` 可生成裂变海报文案、召回文案、朋友圈推广文案。
- 没有配置 API Key 时，会自动返回本地模板文案，保证演示稳定。
- 顾客支付成功页已增加分享领券海报入口，先使用模板二维码和文案，后续可接入真实裂变追踪。

## 上线前检查清单

1. `AI_ENABLED=true` 只在准备好 Key、额度和计费预警后开启。
2. `AI_API_KEY` 不提交到 GitHub，只放服务器环境变量或部署平台密钥管理。
3. 先在商家端生成一次“今日经营建议”和“短视频脚本”，确认响应速度和内容质量。
4. 设置 `AI_TIMEOUT_SECONDS=20` 左右，避免模型慢响应拖垮商家页面。
5. 如果供应商不可用，页面应提示失败，但业务主链路仍可继续使用。

## 商用前建议补齐

- AI 调用日志：记录商家、场景、模型、token 估算、生成结果。
- 套餐额度：按月限制 AI 生成次数，高级版增加更多额度。
- 缓存策略：同一商家同一天经营建议默认缓存，降低成本。
- 合规提示：AI 文案避免承诺疗效、收益、夸大折扣或诱导分享。
- 裂变追踪：为分享链接生成 `share_code`，记录扫码、领券、下单和奖励发放。
- 成本预警：每天统计调用次数和失败率，超过阈值时暂停非核心 AI 功能。
