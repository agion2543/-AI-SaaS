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

var ErrAIUnavailable = errors.New("AI 经营模块尚未接入真实模型，当前不可用")

// AIProvider isolates model vendors from business services. The application
// currently uses DisabledAIProvider; a real adapter must be intentionally wired
// only after data access, privacy, cost, and human-review requirements are ready.
type AIProvider interface {
	Name() string
	Generate(context.Context, AIRequest) (*AIResult, error)
}

type DisabledAIProvider struct{}

func (DisabledAIProvider) Name() string { return "not_connected" }
func (DisabledAIProvider) Generate(context.Context, AIRequest) (*AIResult, error) {
	return nil, ErrAIUnavailable
}

type AIService struct {
	provider AIProvider
}

// NewAIService always installs the disabled provider, regardless of environment
// variables. This prevents accidental model calls while the product is still a
// scaffold. The config argument is retained for constructor compatibility.
func NewAIService(_ *config.Config) *AIService {
	return NewAIServiceWithProvider(DisabledAIProvider{})
}

// NewAIServiceWithProvider is the future integration seam. The normal app
// bootstrap must keep using NewAIService until a separately approved rollout.
func NewAIServiceWithProvider(provider AIProvider) *AIService {
	if provider == nil {
		provider = DisabledAIProvider{}
	}
	return &AIService{provider: provider}
}

func (s *AIService) Generate(ctx context.Context, req AIRequest) (*AIResult, error) {
	if s == nil || s.provider == nil {
		return nil, ErrAIUnavailable
	}
	return s.provider.Generate(ctx, req)
}

type AIRequest struct {
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
	// Model 非空时覆盖配置默认模型（预算降级策略用）；空则用配置模型。
	Model string `json:"model,omitempty"`
}

type AIResult struct {
	Provider           string            `json:"provider"`
	Model              string            `json:"model"`
	Content            string            `json:"content"`
	Structured         []AIOutputSection `json:"structured,omitempty"`
	Fallback           bool              `json:"fallback"`
	PromptTokens       int               `json:"prompt_tokens"`
	CompletionTokens   int               `json:"completion_tokens"`
	CachedTokens       int               `json:"cached_tokens"`
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

// OpenAICompatibleAIProvider is a dormant adapter for a future, explicit
// integration. It is not created or wired by the application bootstrap.
type OpenAICompatibleAIProvider struct {
	cfg    *config.Config
	client *http.Client
}

func NewOpenAICompatibleAIProvider(cfg *config.Config) *OpenAICompatibleAIProvider {
	timeout := 20 * time.Second
	if cfg != nil && cfg.AITimeoutSeconds > 0 {
		timeout = time.Duration(cfg.AITimeoutSeconds) * time.Second
	}
	return &OpenAICompatibleAIProvider{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

func (p *OpenAICompatibleAIProvider) Name() string {
	if p == nil || p.cfg == nil || strings.TrimSpace(p.cfg.AIProvider) == "" {
		return "openai-compatible"
	}
	return strings.TrimSpace(p.cfg.AIProvider)
}

func (p *OpenAICompatibleAIProvider) Generate(ctx context.Context, req AIRequest) (*AIResult, error) {
	if p == nil || p.cfg == nil {
		return nil, errors.New("AI provider is not configured")
	}
	if !p.cfg.AIEnabled || strings.TrimSpace(p.cfg.AIAPIKey) == "" || strings.TrimSpace(p.cfg.AIBaseURL) == "" || strings.TrimSpace(p.cfg.AIModel) == "" {
		return nil, ErrAIUnavailable
	}
	if p.client == nil {
		p.client = NewOpenAICompatibleAIProvider(p.cfg).client
	}

	payload := struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
	}{
		Model: modelName(req.Model, p.cfg),
		Messages: []chatMessage{
			{Role: "system", Content: strings.TrimSpace(req.SystemPrompt)},
			{Role: "user", Content: strings.TrimSpace(req.UserPrompt)},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(p.cfg.AIBaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.AIAPIKey)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai provider returned %d", resp.StatusCode)
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
	cachedTokens := 0
	totalTokens := promptTokens + completionTokens
	if decoded.Usage != nil {
		if decoded.Usage.PromptTokens > 0 {
			promptTokens = decoded.Usage.PromptTokens
		}
		if decoded.Usage.CompletionTokens > 0 {
			completionTokens = decoded.Usage.CompletionTokens
		}
		if decoded.Usage.PromptTokensDetails != nil && decoded.Usage.PromptTokensDetails.CachedTokens > 0 {
			cachedTokens = decoded.Usage.PromptTokensDetails.CachedTokens
		}
		if decoded.Usage.TotalTokens > 0 {
			totalTokens = decoded.Usage.TotalTokens
		} else {
			totalTokens = promptTokens + completionTokens
		}
	}
	provider := p.Name()
	modelName := modelName(req.Model, p.cfg)
	return &AIResult{
		Provider:           provider,
		Model:              modelName,
		Content:            content,
		Fallback:           false,
		PromptTokens:       promptTokens,
		CompletionTokens:   completionTokens,
		CachedTokens:       cachedTokens,
		TotalTokens:        totalTokens,
		EstimatedCostCents: estimateAICostCents(provider, modelName, promptTokens, completionTokens),
	}, nil
}

// modelName 返回请求指定的模型（预算降级用），否则返回配置模型。
func modelName(requestModel string, cfg *config.Config) string {
	if strings.TrimSpace(requestModel) != "" {
		return strings.TrimSpace(requestModel)
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.AIModel)
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
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func approximateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := len([]rune(text)) / 2
	if count <= 0 {
		return 1
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
