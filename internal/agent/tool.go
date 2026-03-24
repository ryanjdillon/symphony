package agent

import (
	"context"
	"encoding/json"
)

// ToolSpec describes a tool that can be advertised to the agent.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolResult is the response sent back to the agent after a tool call.
type ToolResult struct {
	Success      bool          `json:"success"`
	Output       string        `json:"output"`
	ContentItems []ContentItem `json:"contentItems,omitempty"`
}

// ContentItem represents a content block in a tool result.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolHandler processes tool calls from the agent.
type ToolHandler interface {
	// Spec returns the tool specification for registration with the agent.
	Spec() ToolSpec

	// Execute runs the tool with the given arguments and returns the result.
	Execute(ctx context.Context, arguments json.RawMessage) ToolResult
}

// SuccessResult creates a successful tool result from a JSON-serializable value.
func SuccessResult(output any) ToolResult {
	data, err := json.Marshal(output)
	if err != nil {
		return FailureResult("failed to marshal tool output: " + err.Error())
	}
	text := string(data)
	return ToolResult{
		Success: true,
		Output:  text,
		ContentItems: []ContentItem{
			{Type: "inputText", Text: text},
		},
	}
}

// FailureResult creates a failed tool result with an error message.
func FailureResult(errMsg string) ToolResult {
	return ToolResult{
		Success: false,
		Output:  errMsg,
		ContentItems: []ContentItem{
			{Type: "inputText", Text: errMsg},
		},
	}
}
