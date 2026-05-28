package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-web-gin-health/internal/config"
)

type AIService struct {
	cfg    *config.Config
	client *http.Client
}

type AIRequest struct {
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
}

type AIResult struct {
	Provider           string            `json:"provider"`
	Model              string            `json:"model"`
	Content            string            `json:"content"`
	Structured         []AIOutputSection `json:"structured,omitempty"`
	Fallback           bool              `json:"fallback"`
	PromptTokens       int               `json:"prompt_tokens"`
	CompletionTokens   int               `json:"completion_tokens"`
	TotalTokens        int               `json:"total_tokens"`
	EstimatedCostCents int64             `json:"estimated_cost_cents"`
}

type AIOutputSection struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Copy   string `json:"copy"`
	Action string `json:"action"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAIService(cfg *config.Config) *AIService {
	timeout := time.Duration(cfg.AITimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &AIService{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *AIService) Generate(ctx context.Context, req AIRequest) (*AIResult, error) {
	if s == nil || s.cfg == nil {
		return nil, errors.New("ai service is not initialized")
	}
	if !s.cfg.AIEnabled || strings.TrimSpace(s.cfg.AIAPIKey) == "" || strings.TrimSpace(s.cfg.AIBaseURL) == "" {
		content := fallbackMarketingCopy(req.UserPrompt)
		return &AIResult{
			Provider:         s.provider(),
			Model:            s.cfg.AIModel,
			Content:          content,
			Fallback:         true,
			PromptTokens:     approximateTokens(req.SystemPrompt + "\n" + req.UserPrompt),
			CompletionTokens: approximateTokens(content),
			TotalTokens:      approximateTokens(req.SystemPrompt + "\n" + req.UserPrompt + "\n" + content),
		}, nil
	}

	payload := chatRequest{
		Model: strings.TrimSpace(s.cfg.AIModel),
		Messages: []chatMessage{
			{Role: "system", Content: strings.TrimSpace(req.SystemPrompt)},
			{Role: "user", Content: strings.TrimSpace(req.UserPrompt)},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(s.cfg.AIBaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.AIAPIKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai provider returned %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded chatResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return nil, errors.New(decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, errors.New("ai provider returned empty content")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	promptTokens := approximateTokens(req.SystemPrompt + "\n" + req.UserPrompt)
	completionTokens := approximateTokens(content)
	totalTokens := promptTokens + completionTokens
	if decoded.Usage != nil {
		if decoded.Usage.PromptTokens > 0 {
			promptTokens = decoded.Usage.PromptTokens
		}
		if decoded.Usage.CompletionTokens > 0 {
			completionTokens = decoded.Usage.CompletionTokens
		}
		if decoded.Usage.TotalTokens > 0 {
			totalTokens = decoded.Usage.TotalTokens
		} else {
			totalTokens = promptTokens + completionTokens
		}
	}
	provider := s.provider()
	modelName := strings.TrimSpace(s.cfg.AIModel)
	return &AIResult{
		Provider:           provider,
		Model:              modelName,
		Content:            content,
		Fallback:           false,
		PromptTokens:       promptTokens,
		CompletionTokens:   completionTokens,
		TotalTokens:        totalTokens,
		EstimatedCostCents: estimateAICostCents(provider, modelName, promptTokens, completionTokens),
	}, nil
}

func (s *AIService) provider() string {
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.AIProvider) == "" {
		return "template"
	}
	return strings.TrimSpace(s.cfg.AIProvider)
}

func approximateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := len([]rune(text)) / 2
	if count <= 0 {
		count = 1
	}
	return count
}

func estimateAICostCents(provider, model string, promptTokens, completionTokens int) int64 {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(strings.TrimSpace(model))
	inputRateCentsPerMillion := int64(200)
	outputRateCentsPerMillion := int64(800)
	switch {
	case strings.Contains(provider, "qwen") || strings.Contains(model, "qwen-plus"):
		inputRateCentsPerMillion = 80
		outputRateCentsPerMillion = 200
	case strings.Contains(model, "qwen-turbo"):
		inputRateCentsPerMillion = 30
		outputRateCentsPerMillion = 60
	case strings.Contains(provider, "deepseek") && strings.Contains(model, "reasoner"):
		inputRateCentsPerMillion = 400
		outputRateCentsPerMillion = 1600
	case strings.Contains(provider, "deepseek"):
		inputRateCentsPerMillion = 200
		outputRateCentsPerMillion = 800
	case strings.Contains(provider, "moonshot") || strings.Contains(provider, "kimi"):
		inputRateCentsPerMillion = 300
		outputRateCentsPerMillion = 1200
	}
	cost := ceilTokenCost(promptTokens, inputRateCentsPerMillion) + ceilTokenCost(completionTokens, outputRateCentsPerMillion)
	if cost < 0 {
		return 0
	}
	return cost
}

func ceilTokenCost(tokens int, rateCentsPerMillion int64) int64 {
	if tokens <= 0 || rateCentsPerMillion <= 0 {
		return 0
	}
	return (int64(tokens)*rateCentsPerMillion + 999999) / 1000000
}

func fallbackMarketingCopy(userPrompt string) string {
	lowerPrompt := strings.ToLower(userPrompt)
	if strings.Contains(lowerPrompt, "product") || strings.Contains(lowerPrompt, "menu") || strings.Contains(lowerPrompt, "bundle") {
		return `【AI 商品优化建议】

商品标题建议：把商品名称改成“招牌主推 + 规格/场景”的结构，例如“招牌双人套餐”“午市轻食组合”“门店热卖单品”。

商品描述草稿：突出商品适合谁、解决什么需求、口味/规格/服务内容是什么。描述控制在 40-80 字，避免夸大承诺，适合直接填入顾客扫码页。

主推理由：优先把有价格、有图片、描述完整、履约稳定的商品排到前面；缺图或库存不稳定的商品先不要放在首屏。

排序建议：将招牌、套餐、高毛利和复购商品排序值调到 10-30；缺图、售罄、低库存商品暂时排后或下架维护。

套餐建议：选择 1 个主商品 + 1 个高频搭配商品组成轻套餐，先用小幅优惠测试，不要一次性做过大折扣。

执行步骤：
1. 先补齐前 3 个主推商品的图片、描述和价格。
2. 把主推商品排序调到 10、20、30。
3. 对售罄或低库存商品先下架或标注库存。
4. 观察 3-7 天点击、下单和退款情况，再决定是否扩大套餐。

风险提醒：商品描述不要承诺无法稳定履约的服务；套餐优惠先小范围测试，避免利润被折扣吃掉。`
	}
	target := "新客与复购顾客"
	if strings.Contains(userPrompt, "沉睡") || strings.Contains(userPrompt, "召回") {
		target = "沉睡顾客"
	} else if strings.Contains(userPrompt, "高价值") {
		target = "高价值顾客"
	} else if strings.Contains(userPrompt, "附近") {
		target = "附近潜在顾客"
	}

	if strings.Contains(userPrompt, "短视频") || strings.Contains(userPrompt, "脚本") || strings.Contains(userPrompt, "口播") || strings.Contains(strings.ToLower(userPrompt), "video") {
		return fmt.Sprintf(`【AI 短视频脚本】
目标人群：%s

1. 视频标题
“附近这家店，懂的人已经开始偷偷复购了”

2. 前 3 秒钩子
镜头快速切到门店招牌和热销商品，口播：“如果你也在附近，今天这份福利别错过。”

3. 分镜脚本
- 0-3 秒：展示门店门头、热气、出餐动作，字幕突出“附近可到店 / 扫码领券”。
- 4-12 秒：拍 2-3 个主推商品特写，强调新鲜、现做、价格或服务体验。
- 13-22 秒：展示顾客扫码下单和优惠券抵扣，降低第一次消费门槛。
- 23-30 秒：老板或店员出镜提醒：“扫码领券，今天到店直接用。”

4. 口播文案
“这家店适合想省心又想吃好的朋友。现在扫码进店页可以领券，下单还能给朋友分享奖励，下次来继续抵扣。”

5. 发布文案
附近的朋友看过来，今天扫码领券，到店直接用。收藏这条，晚上别再纠结吃什么。

6. 话题建议
#本地生活 #附近美食 #同城探店 #扫码领券 #周末去哪

7. 执行动作
发布后把评论区置顶为门店扫码链接，店内桌贴同步放门店二维码，统计扫码、领券、下单和核销数据。`, target)
	}

	return fmt.Sprintf(`【AI 营销建议】
目标人群：%s

活动标题：老客带新客，扫码立享门店福利

海报主文案：把这家好店分享给朋友，好友扫码领券下单，你也能获得下次到店奖励。

优惠建议：新客首单满 39 减 5 元，分享人奖励 5 元复购券。先用小额券测试，不要一开始把优惠力度拉得过大。

执行动作：
1. 顾客支付成功页展示分享海报。
2. 好友扫码进入门店页领券。
3. 好友下单后自动记录转介绍来源。
4. 分享人获得复购奖励券，引导下次消费。

风险提醒：活动周期建议 7 天，先观察领券率、核销率、客单价和退款率，再决定是否扩大投放。`, target)
}
