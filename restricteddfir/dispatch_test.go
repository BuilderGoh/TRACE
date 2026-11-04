package restricteddfir

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDispatchSummarizesSyntheticEventsDeterministically(t *testing.T) {
	input := []byte(`{"synthetic":true,"events":[{"type":"login"},{"type":"file-write"},{"type":"login"}]}`)
	want := Summary{
		EventCount: 3,
		ByType: []TypeCount{
			{Type: "file-write", Count: 1},
			{Type: "login", Count: 2},
		},
	}
	for i := 0; i < 2; i++ {
		got, err := Dispatch(AnalyzeSyntheticEvents, input)
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Dispatch() = %#v, want %#v", got, want)
		}
	}
}

func TestDispatchDeniesUnknownToolsByDefault(t *testing.T) {
	for _, name := range []string{"", "Bash", "read_file", "mcp__host__tool"} {
		if _, err := Dispatch(name, []byte(`{"synthetic":true,"events":[]}`)); !errors.Is(err, ErrToolDenied) {
			t.Errorf("Dispatch(%q) error = %v, want ErrToolDenied", name, err)
		}
	}
}

func TestDispatchRequiresExplicitSyntheticLabel(t *testing.T) {
	for _, input := range []string{
		`{"synthetic":false,"events":[]}`,
		`{"events":[]}`,
	} {
		if _, err := Dispatch(AnalyzeSyntheticEvents, []byte(input)); !errors.Is(err, ErrNotSynthetic) {
			t.Errorf("Dispatch(%s) error = %v, want ErrNotSynthetic", input, err)
		}
	}
}

func TestDispatchRejectsMalformedOrExpandedInput(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  error
	}{
		{"malformed JSON", []byte(`{`), ErrInvalidInput},
		{"trailing JSON", []byte(`{"synthetic":true,"events":[]} {}`), ErrInvalidInput},
		{"unknown field", []byte(`{"synthetic":true,"events":[],"command":"echo"}`), ErrInvalidInput},
		{"missing events", []byte(`{"synthetic":true}`), ErrInvalidInput},
		{"null events", []byte(`{"synthetic":true,"events":null}`), ErrInvalidInput},
		{"oversize", []byte(strings.Repeat("x", maxInputBytes+1)), ErrInputTooLarge},
		{"invalid event type", []byte(`{"synthetic":true,"events":[{"type":"../secret"}]}`), ErrInvalidEventType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Dispatch(AnalyzeSyntheticEvents, tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("Dispatch() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDispatchRejectsTooManyEvents(t *testing.T) {
	input := []byte(`{"synthetic":true,"events":[` + strings.TrimSuffix(strings.Repeat(`{"type":"login"},`, maxEvents+1), ",") + `]}`)
	if _, err := Dispatch(AnalyzeSyntheticEvents, input); !errors.Is(err, ErrTooManyEvents) {
		t.Fatalf("Dispatch() error = %v, want ErrTooManyEvents", err)
	}
}
