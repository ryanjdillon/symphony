package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// AppServerRunner implements the app-server stdio JSON protocol shared by
// Claude Code and Codex. Both agents use the same handshake and event format.
type AppServerRunner struct {
	name    string
	command string
	logger  *slog.Logger
}

// NewAppServerRunner creates a runner for an app-server compatible agent.
func NewAppServerRunner(name, command string, logger *slog.Logger) *AppServerRunner {
	return &AppServerRunner{
		name:    name,
		command: command,
		logger:  logger,
	}
}

func (r *AppServerRunner) Name() string { return r.name }

func (r *AppServerRunner) Start(ctx context.Context, opts *StartOpts) (Session, error) {
	parts := strings.Fields(r.command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty agent command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = opts.WorkspacePath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting agent process: %w", err)
	}

	threadID := uuid.New().String()
	turnID := uuid.New().String()

	s := &appServerSession{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		events:    make(chan Event, 128),
		done:      make(chan struct{}),
		threadID:  threadID,
		turnID:    turnID,
		sessionID: threadID + "-" + turnID,
		logger:    r.logger.With("session_id", threadID+"-"+turnID),
	}

	go s.run(opts)

	return s, nil
}

type appServerSession struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	events    chan Event
	done      chan struct{}
	threadID  string
	turnID    string
	sessionID string
	outcome   Outcome
	mu        sync.Mutex
	logger    *slog.Logger
}

func (s *appServerSession) Events() <-chan Event { return s.events }
func (s *appServerSession) SessionID() string    { return s.sessionID }

func (s *appServerSession) Wait() Outcome {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outcome
}

func (s *appServerSession) Stop() error {
	if s.cmd.Process == nil {
		return nil
	}

	// Graceful: SIGTERM first
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return s.cmd.Process.Kill()
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return s.cmd.Process.Kill()
	}
}

func (s *appServerSession) run(opts *StartOpts) {
	defer close(s.events)
	defer close(s.done)

	scanner := bufio.NewScanner(s.stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB line buffer

	// Handshake: initialize
	if err := s.sendJSON(map[string]string{"type": "initialize", "version": "1.0"}); err != nil {
		s.logger.Error("handshake failed: initialize", "error", err)
		s.setOutcome(Failed)
		return
	}

	if !s.waitForType(scanner, "initialized") {
		s.logger.Error("handshake failed: no initialized response")
		s.setOutcome(Failed)
		return
	}

	// Start thread
	if err := s.sendJSON(map[string]string{"type": "thread/start", "thread_id": s.threadID}); err != nil {
		s.logger.Error("failed to start thread", "error", err)
		s.setOutcome(Failed)
		return
	}

	// Start turn with prompt
	turnMsg := map[string]string{
		"type":    "turn/start",
		"turn_id": s.turnID,
		"prompt":  opts.Prompt,
	}
	if err := s.sendJSON(turnMsg); err != nil {
		s.logger.Error("failed to start turn", "error", err)
		s.setOutcome(Failed)
		return
	}

	// Stream events until session ends
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			s.logger.Warn("unparseable event line", "error", err)
			continue
		}

		var eventType string
		if t, ok := raw["type"]; ok {
			_ = json.Unmarshal(t, &eventType)
		}

		event := Event{
			Type:      eventType,
			Timestamp: time.Now(),
			Payload:   json.RawMessage(line),
		}

		// Extract token usage from event
		s.extractTokens(raw, &event)

		s.events <- event

		switch eventType {
		case "turn/completed":
			s.setOutcome(Succeeded)
			return
		case "turn/failed":
			s.setOutcome(Failed)
			return
		case "turn/cancelled":
			s.setOutcome(CanceledByReconciliation)
			return
		}
	}

	// Process exited without a terminal event
	if err := s.cmd.Wait(); err != nil {
		s.logger.Warn("agent process exited with error", "error", err)
		s.setOutcome(Failed)
	} else {
		s.setOutcome(Succeeded)
	}
}

func (s *appServerSession) sendJSON(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}
	data = append(data, '\n')
	_, err = s.stdin.Write(data)
	return err
}

func (s *appServerSession) waitForType(scanner *bufio.Scanner, expectedType string) bool {
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Type == expectedType {
			return true
		}
	}
	return false
}

func (s *appServerSession) extractTokens(raw map[string]json.RawMessage, event *Event) {
	// Look for usage data in various nested locations
	for _, key := range []string{"usage", "data"} {
		data, ok := raw[key]
		if !ok {
			continue
		}
		var usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		}
		if json.Unmarshal(data, &usage) == nil && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
			event.Tokens = TokenUsage{
				Input:  usage.InputTokens,
				Output: usage.OutputTokens,
				Total:  usage.InputTokens + usage.OutputTokens,
			}
			return
		}
	}
}

func (s *appServerSession) setOutcome(o Outcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcome = o
}
