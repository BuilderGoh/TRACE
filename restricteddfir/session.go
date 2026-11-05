package restricteddfir

import (
	"context"
	"encoding/json"

	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/norma/permission"
	"github.com/Autumn-27/norma/tool"
)

// NewSession constructs a standalone Norma session with exactly one capability:
// analyze_synthetic_events. TRACE's dedicated synthetic-analysis endpoint uses
// it per request; it is not attached to existing agents. The caller owns the
// session and should Close it.
func NewSession(provider llm.Provider) *agentcore.Session {
	analyzer := tool.Build(tool.Spec{
		Name:        AnalyzeSyntheticEvents,
		Description: "Summarize explicitly labelled synthetic event records.",
		Prompt:      "Accepts only synthetic:true and returns deterministic event counts.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"synthetic": map[string]any{"type": "boolean", "const": true},
				"events": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"type": map[string]any{"type": "string"}},
						"required":             []string{"type"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"synthetic", "events"},
			"additionalProperties": false,
		},
		ReadOnly:   func(json.RawMessage) bool { return true },
		Concurrent: func(json.RawMessage) bool { return false },
		Permissions: func(context.Context, json.RawMessage, permission.Context) permission.Decision {
			return permission.Allowed()
		},
		Run: func(_ context.Context, input json.RawMessage, _ *tool.ToolContext) (tool.Result, error) {
			summary, err := Dispatch(AnalyzeSyntheticEvents, input)
			if err != nil {
				return tool.Errorf(err.Error()), nil
			}
			encoded, err := json.Marshal(summary)
			if err != nil {
				return tool.Errorf("restricteddfir: encode summary: " + err.Error()), nil
			}
			return tool.Text(string(encoded)), nil
		},
	})

	return agentcore.NewSession(agentcore.Options{
		Provider:               provider,
		SystemPrompt:           []string{"Analyze only explicitly synthetic event data using analyze_synthetic_events."},
		Tools:                  []tool.CoreTool{analyzer},
		DisableBackgroundTasks: true,
		NonStreaming:           true,
	})
}
