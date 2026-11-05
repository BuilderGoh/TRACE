package restricteddfir

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
	"testing"

	"github.com/Autumn-27/norma/harness"
	"github.com/Autumn-27/norma/llm"
)

type scriptedProvider struct {
	responses []llm.Message
	requests  []llm.CompletionRequest
}

func (p *scriptedProvider) Stream(_ context.Context, _ llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) {
		yield(llm.StreamEvent{}, fmt.Errorf("unexpected streaming call"))
	}
}

func (p *scriptedProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.requests = append(p.requests, req)
	if len(p.responses) == 0 {
		return llm.Message{}, "", llm.Usage{}, fmt.Errorf("unexpected model call")
	}
	response := p.responses[0]
	p.responses = p.responses[1:]
	if len(response.ToolUses()) > 0 {
		return response, "tool_use", llm.Usage{}, nil
	}
	return response, "end_turn", llm.Usage{}, nil
}

func toolCall(name, input string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{
		Type: llm.BlockToolUse, ID: "call-1", Name: name, Input: json.RawMessage(input),
	}}}
}

func runSession(t *testing.T, calls ...llm.Message) ([]llm.CompletionRequest, []llm.Message) {
	t.Helper()
	provider := &scriptedProvider{responses: append(calls, llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock("done")}})}
	session := NewSession(provider)
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	var events []harness.Event
	for event, err := range session.Prompt(context.Background(), "analyze these synthetic events") {
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		events = append(events, event)
	}
	return provider.requests, session.Messages()
}

func TestSessionAdvertisesOnlySyntheticAnalyzerAndRunsIt(t *testing.T) {
	requests, messages := runSession(t, toolCall(AnalyzeSyntheticEvents, `{"synthetic":true,"events":[{"type":"login"},{"type":"login"},{"type":"file-write"}]}`))
	if len(requests) != 2 {
		t.Fatalf("model requests = %d, want initial tool call plus final response", len(requests))
	}
	if got := len(requests[0].Tools); got != 1 || requests[0].Tools[0].Name != AnalyzeSyntheticEvents {
		t.Fatalf("effective advertised tools = %#v, want only %q", requests[0].Tools, AnalyzeSyntheticEvents)
	}
	var results []llm.ContentBlock
	for _, message := range messages {
		for _, block := range message.Content {
			if block.Type == llm.BlockToolResult {
				results = append(results, block)
			}
		}
	}
	if len(results) != 1 || results[0].IsError {
		t.Fatalf("tool results = %#v, want one successful analyzer result", results)
	}
	if got, want := results[0].Content[0].Text, `{"event_count":3,"by_type":[{"type":"file-write","count":1},{"type":"login","count":2}]}`; got != want {
		t.Fatalf("analyzer result = %q, want %q", got, want)
	}
}

func TestSessionRejectsUnregisteredAndOptionalCapabilities(t *testing.T) {
	names := []string{
		"unknown_tool", "Bash", "WebFetch", "WebSearch", "mcp__host__tool",
		"TaskOutput", "TaskStop", "TaskList", "Monitor", "SearchExtraTools", "ExecuteExtraTool",
	}
	calls := make([]llm.Message, 0, len(names))
	for _, name := range names {
		calls = append(calls, toolCall(name, `{}`))
	}
	requests, messages := runSession(t, calls...)
	if len(requests) != len(names)+1 {
		t.Fatalf("model requests = %d, want %d tool attempts and final response", len(requests), len(names)+1)
	}
	for i, req := range requests {
		if len(req.Tools) != 1 || req.Tools[0].Name != AnalyzeSyntheticEvents {
			t.Fatalf("request %d effective tools = %#v", i, req.Tools)
		}
	}
	var gotErrors []string
	for _, message := range messages {
		for _, block := range message.Content {
			if block.Type == llm.BlockToolResult {
				if !block.IsError || len(block.Content) != 1 {
					t.Fatalf("unregistered call result = %#v, want error result", block)
				}
				gotErrors = append(gotErrors, block.Content[0].Text)
			}
		}
	}
	wantErrors := make([]string, len(names))
	for i, name := range names {
		wantErrors[i] = fmt.Sprintf("Error: unknown tool %q", name)
	}
	if !reflect.DeepEqual(gotErrors, wantErrors) {
		t.Fatalf("tool errors = %#v, want %#v", gotErrors, wantErrors)
	}
}

func TestSessionRejectsMalformedAnalyzerInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"malformed JSON", `{`},
		{"invalid semantic event", `{"synthetic":true,"events":[{"type":"../secret"}]}`},
		{"false synthetic label", `{"synthetic":false,"events":[]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests, messages := runSession(t, toolCall(AnalyzeSyntheticEvents, test.input))
			if len(requests) != 2 || len(requests[0].Tools) != 1 {
				t.Fatalf("requests = %#v, expected restricted tool request and final response", requests)
			}
			for _, message := range messages {
				for _, block := range message.Content {
					if block.Type == llm.BlockToolResult && !block.IsError {
						t.Fatalf("malformed analyzer input unexpectedly succeeded: %#v", block)
					}
				}
			}
		})
	}
}

func TestSessionDoesNotFallbackAfterAnalyzerToolError(t *testing.T) {
	requests, messages := runSession(t,
		toolCall(AnalyzeSyntheticEvents, `{"synthetic":true,"events":[{"type":"../secret"}]}`),
		toolCall("Bash", `{"command":"echo unrestricted"}`),
	)
	if len(requests) != 3 {
		t.Fatalf("model requests = %d, want two tool attempts plus final response", len(requests))
	}
	var results []llm.ContentBlock
	for _, message := range messages {
		for _, block := range message.Content {
			if block.Type == llm.BlockToolResult {
				results = append(results, block)
			}
		}
	}
	if len(results) != 2 || !results[0].IsError || !results[1].IsError || results[1].Content[0].Text != `Error: unknown tool "Bash"` {
		t.Fatalf("tool results = %#v, want analyzer error and denied Bash, with no fallback", results)
	}
}
