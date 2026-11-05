package restricteddfir

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
)

const (
	EndpointPath   = "/api/dfir/synthetic/analyze"
	FeatureFlagEnv = "TRACE_RESTRICTED_DFIR_ENABLED"
	requestTimeout = 30 * time.Second
)

// ServeHTTP accepts only the bounded synthetic dataset schema, asks the
// restricted session to run its sole analyzer, and returns that analyzer's
// deterministic result. The supplied provider is used only for this request.
func ServeHTTP(w http.ResponseWriter, r *http.Request, enabled bool, provider llm.Provider) {
	if !enabled {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if provider == nil {
		http.Error(w, "restricted synthetic analyzer unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxInputBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, ErrInputTooLarge.Error(), http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request body", http.StatusBadRequest)
		}
		return
	}
	expected, err := Dispatch(AnalyzeSyntheticEvents, raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	session := NewSession(provider)
	defer session.Close()

	prompt := "Call analyze_synthetic_events once with exactly this JSON dataset and return no other analysis: " + string(raw)
	for _, promptErr := range session.Prompt(ctx, prompt) {
		if promptErr != nil {
			if ctx.Err() != nil {
				http.Error(w, "restricted synthetic analysis cancelled or timed out", http.StatusGatewayTimeout)
				return
			}
			http.Error(w, "restricted synthetic analysis failed", http.StatusBadGateway)
			return
		}
	}

	if ctx.Err() != nil {
		http.Error(w, "restricted synthetic analysis cancelled or timed out", http.StatusGatewayTimeout)
		return
	}
	if !verifiedAnalyzerCall(session, expected) {
		http.Error(w, "restricted synthetic analyzer did not produce a verified result", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(expected)
}

func verifiedAnalyzerCall(session *agentcore.Session, expected Summary) bool {
	var callID string
	calls := 0
	results := 0
	for _, message := range session.Messages() {
		for _, block := range message.Content {
			switch block.Type {
			case llm.BlockToolUse:
				calls++
				if block.Name != AnalyzeSyntheticEvents || calls != 1 {
					return false
				}
				got, err := Dispatch(AnalyzeSyntheticEvents, block.Input)
				if err != nil || !sameSummary(got, expected) {
					return false
				}
				callID = block.ID
			case llm.BlockToolResult:
				if block.ToolUseID != callID || block.IsError || len(block.Content) != 1 {
					return false
				}
				var got Summary
				if err := json.Unmarshal([]byte(block.Content[0].Text), &got); err != nil || !sameSummary(got, expected) {
					return false
				}
				results++
			}
		}
	}
	return calls == 1 && results == 1
}

func sameSummary(a, b Summary) bool {
	aBytes, errA := json.Marshal(a)
	bBytes, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(aBytes, bBytes)
}
