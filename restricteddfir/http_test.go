package restricteddfir

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

func TestServeHTTPReturnsDeterministicSummary(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.Message{
		toolCall(AnalyzeSyntheticEvents, `{"synthetic":true,"events":[{"type":"login"},{"type":"file-write"},{"type":"login"}]}`),
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock("analysis complete")}},
	}}
	req := httptest.NewRequest(http.MethodPost, EndpointPath, strings.NewReader(`{"synthetic":true,"events":[{"type":"login"},{"type":"file-write"},{"type":"login"}]}`))
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req, true, provider)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"event_count":3,"by_type":[{"type":"file-write","count":1},{"type":"login","count":2}]}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want one tool call and one final response", len(provider.requests))
	}
}

func TestServeHTTPDisabledAndRejectsInvalidInputBeforeProviderCall(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		body    string
		status  int
	}{
		{"disabled", false, `{"synthetic":true,"events":[]}`, http.StatusNotFound},
		{"invalid dataset", true, `{"synthetic":false,"events":[]}`, http.StatusBadRequest},
		{"oversized body", true, strings.Repeat(" ", maxInputBytes+1), http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &scriptedProvider{}
			req := httptest.NewRequest(http.MethodPost, EndpointPath, strings.NewReader(test.body))
			rec := httptest.NewRecorder()
			ServeHTTP(rec, req, test.enabled, provider)
			if rec.Code != test.status {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, test.status, rec.Body.String())
			}
			if len(provider.requests) != 0 {
				t.Fatalf("provider called %d times for rejected request", len(provider.requests))
			}
		})
	}
}

func TestServeHTTPHonorsCancelledRequest(t *testing.T) {
	provider := &scriptedProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, EndpointPath, strings.NewReader(`{"synthetic":true,"events":[]}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	ServeHTTP(rec, req, true, provider)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusGatewayTimeout, rec.Body.String())
	}
}
