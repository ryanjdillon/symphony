package agent

import (
	"encoding/json"
	"testing"
)

func TestSuccessResult(t *testing.T) {
	result := SuccessResult(map[string]string{"key": "value"})
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
	if len(result.ContentItems) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.ContentItems))
	}
	if result.ContentItems[0].Type != "inputText" {
		t.Errorf("content item type = %q, want inputText", result.ContentItems[0].Type)
	}

	// Verify output is valid JSON
	var parsed map[string]string
	if err := json.Unmarshal([]byte(result.Output), &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("parsed key = %q, want %q", parsed["key"], "value")
	}
}

func TestFailureResult(t *testing.T) {
	result := FailureResult("something went wrong")
	if result.Success {
		t.Error("expected success=false")
	}
	if result.Output != "something went wrong" {
		t.Errorf("output = %q, want %q", result.Output, "something went wrong")
	}
	if len(result.ContentItems) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.ContentItems))
	}
}
