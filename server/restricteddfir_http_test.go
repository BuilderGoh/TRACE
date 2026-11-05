package server

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/artex/restricteddfir"
)

type restrictedDFIRTestProvider struct {
	responses []llm.Message
	requests  []llm.CompletionRequest
	err       error
}

func (p *restrictedDFIRTestProvider) Stream(context.Context, llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) {
		yield(llm.StreamEvent{}, errors.New("unexpected streaming request"))
	}
}

func (p *restrictedDFIRTestProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.requests = append(p.requests, req)
	if p.err != nil {
		return llm.Message{}, "", llm.Usage{}, p.err
	}
	if len(p.responses) == 0 {
		return llm.Message{}, "", llm.Usage{}, errors.New("unexpected model request")
	}
	response := p.responses[0]
	p.responses = p.responses[1:]
	if len(response.ToolUses()) != 0 {
		return response, "tool_use", llm.Usage{}, nil
	}
	return response, "end_turn", llm.Usage{}, nil
}

func TestRestrictedDFIRRouteRequiresAuthentication(t *testing.T) {
	t.Setenv(restricteddfir.FeatureFlagEnv, "true")
	provider := &restrictedDFIRTestProvider{}
	s := &Server{jwtKey: []byte("restricted-dfir-server-test-key"), llmDirect: provider}
	req := httptest.NewRequest(http.MethodPost, restricteddfir.EndpointPath, strings.NewReader(`{"synthetic":true,"events":[]}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d (body=%q)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if len(provider.requests) != 0 {
		t.Fatalf("unauthenticated request reached provider %d times", len(provider.requests))
	}
}

func TestRestrictedDFIRRouteUsesDirectProviderOnly(t *testing.T) {
	t.Setenv(restricteddfir.FeatureFlagEnv, "true")
	direct := &restrictedDFIRTestProvider{responses: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{
			Type: llm.BlockToolUse,
			ID:   "restricted-dfir-call",
			Name: restricteddfir.AnalyzeSyntheticEvents,
			Input: json.RawMessage(`{"synthetic":true,"events":[{"type":"login"}]}`),
		}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock("complete")}},
	}}
	fallback := &restrictedDFIRTestProvider{err: errors.New("failover provider must not be used")}
	s := &Server{jwtKey: []byte("restricted-dfir-server-test-key"), llmDirect: direct, llmProv: fallback}
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, restricteddfir.EndpointPath, strings.NewReader(`{"synthetic":true,"events":[{"type":"login"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"event_count":1,"by_type":[{"type":"login","count":1}]}`; got != want {
		t.Fatalf("response = %s, want %s", got, want)
	}
	if len(direct.requests) != 2 {
		t.Fatalf("direct provider calls = %d, want 2", len(direct.requests))
	}
	for i, req := range direct.requests {
		if len(req.Tools) != 1 || req.Tools[0].Name != restricteddfir.AnalyzeSyntheticEvents {
			t.Fatalf("direct request %d advertised tools = %#v", i, req.Tools)
		}
	}
	if len(fallback.requests) != 0 {
		t.Fatalf("failover provider called %d times", len(fallback.requests))
	}
}

func TestRestrictedDFIRRouteDisabledByDefault(t *testing.T) {
	t.Setenv(restricteddfir.FeatureFlagEnv, "")
	provider := &restrictedDFIRTestProvider{}
	s := &Server{jwtKey: []byte("restricted-dfir-server-test-key"), llmDirect: provider}
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, restricteddfir.EndpointPath, strings.NewReader(`{"synthetic":true,"events":[]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled route status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("disabled route reached provider %d times", len(provider.requests))
	}
}
