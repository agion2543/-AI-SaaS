package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go-web-gin-health/internal/config"
)

func TestNewAIServiceDefaultsToDisabledProviderWithoutFallbackOrNetwork(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"must not be used"}}]}`))
	}))
	defer server.Close()

	service := NewAIService(&config.Config{
		AIEnabled:        true,
		AIProvider:       "synthetic-provider",
		AIBaseURL:        server.URL,
		AIAPIKey:         "synthetic-test-key",
		AIModel:          "synthetic-model",
		AITimeoutSeconds: 1,
	})
	result, err := service.Generate(context.Background(), AIRequest{
		SystemPrompt: "synthetic system prompt",
		UserPrompt:   "synthetic user prompt",
	})
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("default app service should report AI unavailable, got result=%#v err=%v", result, err)
	}
	if result != nil {
		t.Fatalf("disabled service must not fabricate or return a result: %#v", result)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("disabled default provider must not make any HTTP request, got %d", got)
	}
}

func TestDisabledAIProviderReturnsExplicitUnavailableError(t *testing.T) {
	result, err := (DisabledAIProvider{}).Generate(context.Background(), AIRequest{UserPrompt: "synthetic prompt"})
	if !errors.Is(err, ErrAIUnavailable) || result != nil {
		t.Fatalf("disabled provider should return only the unavailable signal, result=%#v err=%v", result, err)
	}
}
