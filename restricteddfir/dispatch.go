// Package restricteddfir contains a bounded synthetic-event analyzer exposed
// through a dedicated opt-in server endpoint. It is not wired into TRACE's
// agent or ordinary execution paths and does not establish a security boundary
// for them.
package restricteddfir

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	// AnalyzeSyntheticEvents is the only tool name accepted by Dispatch.
	AnalyzeSyntheticEvents = "analyze_synthetic_events"
	maxInputBytes          = 1 << 20
	maxEvents              = 10000
	maxEventTypeBytes      = 64
)

var (
	ErrToolDenied       = errors.New("restricteddfir: tool denied")
	ErrNotSynthetic     = errors.New("restricteddfir: input is not explicitly labelled synthetic")
	ErrInvalidInput     = errors.New("restricteddfir: invalid input")
	ErrInputTooLarge    = errors.New("restricteddfir: input exceeds size limit")
	ErrTooManyEvents    = errors.New("restricteddfir: event count exceeds limit")
	ErrInvalidEventType = errors.New("restricteddfir: invalid event type")
)

// Event is a deliberately small synthetic telemetry record. No event fields
// carry paths, commands, network destinations, or executable content.
type Event struct {
	Type string `json:"type"`
}

// Dataset requires the caller to explicitly label the records as synthetic.
// The label is an input gate, not provenance verification.
type Dataset struct {
	Synthetic bool    `json:"synthetic"`
	Events    []Event `json:"events"`
}

type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Summary is the deterministic output of the sole permitted analysis.
type Summary struct {
	EventCount int         `json:"event_count"`
	ByType     []TypeCount `json:"by_type"`
}

// Dispatch is default-deny: only the fixed synthetic summarizer is recognized.
// It performs no filesystem, process, network, agent, or MCP operations.
func Dispatch(tool string, input []byte) (Summary, error) {
	if tool != AnalyzeSyntheticEvents {
		return Summary{}, fmt.Errorf("%w: %q", ErrToolDenied, tool)
	}
	if len(input) > maxInputBytes {
		return Summary{}, ErrInputTooLarge
	}

	var dataset Dataset
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dataset); err != nil {
		return Summary{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Summary{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidInput)
	}
	if !dataset.Synthetic {
		return Summary{}, ErrNotSynthetic
	}
	if dataset.Events == nil {
		return Summary{}, fmt.Errorf("%w: events must be an array", ErrInvalidInput)
	}
	if len(dataset.Events) > maxEvents {
		return Summary{}, ErrTooManyEvents
	}

	counts := make(map[string]int)
	for _, event := range dataset.Events {
		if !validEventType(event.Type) {
			return Summary{}, ErrInvalidEventType
		}
		counts[event.Type]++
	}

	types := make([]string, 0, len(counts))
	for eventType := range counts {
		types = append(types, eventType)
	}
	sort.Strings(types)
	byType := make([]TypeCount, 0, len(types))
	for _, eventType := range types {
		byType = append(byType, TypeCount{Type: eventType, Count: counts[eventType]})
	}
	return Summary{EventCount: len(dataset.Events), ByType: byType}, nil
}

func validEventType(value string) bool {
	if len(value) == 0 || len(value) > maxEventTypeBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}
