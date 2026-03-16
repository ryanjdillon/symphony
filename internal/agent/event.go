package agent

import (
	"encoding/json"
	"time"
)

// Event represents a streaming event from an agent session.
type Event struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Tokens    TokenUsage      `json:"tokens,omitempty"`
}

// TokenUsage tracks cumulative token consumption for a session.
type TokenUsage struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Total  int64 `json:"total"`
}
