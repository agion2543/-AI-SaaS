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
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Content  string `json:"content"`
	Fallback bool   `json:"fallback"`
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
		return &AIResult{
			Provider: s.provider(),
			Model:    s.cfg.AIModel,
			Content:  fallbackMarketingCopy(req.UserPrompt),
			Fallback: true,
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
	return &AIResult{
		Provider: s.provider(),
		Model:    s.cfg.AIModel,
		Content:  strings.TrimSpace(decoded.Choices[0].Message.Content),
		Fallback: false,
	}, nil
}

func (s *AIService) provider() string {
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.AIProvider) == "" {
		return "template"
	}
	return strings.TrimSpace(s.cfg.AIProvider)
}

func fallbackMarketingCopy(userPrompt string) string {
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
