package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config holds the full parsed workflow configuration.
type Config struct {
	Tracker        TrackerConfig   `yaml:"tracker"`
	Polling        PollingConfig   `yaml:"polling"`
	Workspace      WorkspaceConfig `yaml:"workspace"`
	Agent          AgentConfig     `yaml:"agent"`
	Codex          *CodexConfig    `yaml:"codex,omitempty"`
	Server         ServerConfig    `yaml:"server"`
	PromptTemplate string          `yaml:"-"`
	SourcePath     string          `yaml:"-"`
}

type TrackerConfig struct {
	Kind           string   `yaml:"kind"`
	ProjectSlug    string   `yaml:"project_slug"`
	APIKey         string   `yaml:"api_key"`
	ActiveStates   []string `yaml:"active_states"`
	TerminalStates []string `yaml:"terminal_states"`
}

type PollingConfig struct {
	IntervalMs int `yaml:"interval_ms"`
}

type WorkspaceConfig struct {
	Root  string      `yaml:"root"`
	Hooks HooksConfig `yaml:"hooks"`
}

type HooksConfig struct {
	AfterCreate   string `yaml:"after_create"`
	BeforeRun     string `yaml:"before_run"`
	AfterRun      string `yaml:"after_run"`
	BeforeRemove  string `yaml:"before_remove"`
	HookTimeoutMs int    `yaml:"hook_timeout_ms"`
}

type AgentConfig struct {
	Kind                       string         `yaml:"kind"`
	Command                    string         `yaml:"command"`
	MaxConcurrentAgents        int            `yaml:"max_concurrent_agents"`
	MaxConcurrentAgentsByState map[string]int `yaml:"max_concurrent_agents_by_state"`
	MaxTurns                   int            `yaml:"max_turns"`
	MaxRetryBackoffMs          int            `yaml:"max_retry_backoff_ms"`
	Config                     map[string]any `yaml:"config"`
}

// TurnTimeoutMs returns the turn timeout from agent config, defaulting to 3600000.
func (a AgentConfig) TurnTimeoutMs() int {
	if v, ok := a.Config["turn_timeout_ms"]; ok {
		switch t := v.(type) {
		case int:
			return t
		case float64:
			return int(t)
		}
	}
	return 3600000
}

// StallTimeoutMs returns the stall timeout from agent config, defaulting to 300000.
func (a AgentConfig) StallTimeoutMs() int {
	if v, ok := a.Config["stall_timeout_ms"]; ok {
		switch t := v.(type) {
		case int:
			return t
		case float64:
			return int(t)
		}
	}
	return 300000
}

type CodexConfig struct {
	Command           string `yaml:"command"`
	ApprovalPolicy    string `yaml:"approval_policy"`
	ThreadSandbox     string `yaml:"thread_sandbox"`
	TurnSandboxPolicy any    `yaml:"turn_sandbox_policy"`
	TurnTimeoutMs     int    `yaml:"turn_timeout_ms"`
	StallTimeoutMs    int    `yaml:"stall_timeout_ms"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

// ApplyDefaults fills in missing fields with sensible defaults.
func (c *Config) ApplyDefaults() {
	if len(c.Tracker.ActiveStates) == 0 {
		c.Tracker.ActiveStates = []string{"Todo", "In Progress"}
	}
	if len(c.Tracker.TerminalStates) == 0 {
		c.Tracker.TerminalStates = []string{"Done", "Closed", "Cancelled"}
	}
	if c.Polling.IntervalMs == 0 {
		c.Polling.IntervalMs = 30000
	}
	if c.Workspace.Root == "" {
		c.Workspace.Root = filepath.Join(os.TempDir(), "symphony-workspaces")
	}
	if c.Workspace.Root[:2] == "~/" {
		home, _ := os.UserHomeDir()
		c.Workspace.Root = filepath.Join(home, c.Workspace.Root[2:])
	}
	if c.Workspace.Hooks.HookTimeoutMs == 0 {
		c.Workspace.Hooks.HookTimeoutMs = 60000
	}
	if c.Agent.Kind == "" {
		c.Agent.Kind = "codex"
	}
	if c.Agent.MaxConcurrentAgents == 0 {
		c.Agent.MaxConcurrentAgents = 10
	}
	if c.Agent.MaxTurns == 0 {
		c.Agent.MaxTurns = 20
	}
	if c.Agent.MaxRetryBackoffMs == 0 {
		c.Agent.MaxRetryBackoffMs = 300000
	}
	if c.Agent.Config == nil {
		c.Agent.Config = make(map[string]any)
	}
}

// Validate checks that all required fields are present.
func (c *Config) Validate() error {
	if c.Tracker.Kind == "" {
		return fmt.Errorf("tracker.kind is required")
	}
	if c.Tracker.ProjectSlug == "" {
		return fmt.Errorf("tracker.project_slug is required")
	}
	if c.Tracker.APIKey == "" {
		return fmt.Errorf("tracker.api_key is required")
	}
	if c.Agent.Command == "" {
		return fmt.Errorf("agent.command is required")
	}
	return nil
}

// ResolveEnvVars replaces $VAR_NAME references with environment variable values.
func ResolveEnvVars(s string) string {
	if !strings.HasPrefix(s, "$") {
		return s
	}
	varName := s[1:]
	if val := os.Getenv(varName); val != "" {
		return val
	}
	return s
}

// WatchWorkflow starts a file watcher on the given path and calls onReload
// when the file changes. Returns a stop function.
func WatchWorkflow(path string, onReload func(*Config), logger *slog.Logger) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	if err := watcher.Add(filepath.Dir(absPath)); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watching directory: %w", err)
	}

	done := make(chan struct{})
	go func() {
		var debounce *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != absPath {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(100*time.Millisecond, func() {
					logger.Info("workflow file changed, reloading", "path", path)
					cfg, err := LoadWorkflow(path)
					if err != nil {
						logger.Error("failed to reload workflow", "error", err)
						return
					}
					cfg.ApplyDefaults()
					onReload(cfg)
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("file watcher error", "error", err)
			case <-done:
				return
			}
		}
	}()

	stop := func() {
		close(done)
		_ = watcher.Close()
	}
	return stop, nil
}
