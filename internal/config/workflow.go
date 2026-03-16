package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var frontmatterDelimiter = []byte("---")

// LoadWorkflow reads and parses a WORKFLOW.md file from disk.
func LoadWorkflow(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %s: %w", path, err)
	}

	cfg, promptBody, err := parseWorkflow(data)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow file %s: %w", path, err)
	}
	cfg.PromptTemplate = promptBody
	cfg.SourcePath = path
	return cfg, nil
}

func parseWorkflow(data []byte) (*Config, string, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, "", err
	}

	var cfg Config
	if err := yaml.Unmarshal(frontmatter, &cfg); err != nil {
		return nil, "", fmt.Errorf("unmarshaling YAML frontmatter: %w", err)
	}

	applyCodexCompat(&cfg)

	return &cfg, string(body), nil
}

func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	trimmed := bytes.TrimLeft(data, " \t\r\n")

	if !bytes.HasPrefix(trimmed, frontmatterDelimiter) {
		return nil, nil, fmt.Errorf("workflow file must begin with --- frontmatter delimiter")
	}

	rest := trimmed[len(frontmatterDelimiter):]
	rest = skipNewline(rest)

	idx := bytes.Index(rest, append([]byte("\n"), frontmatterDelimiter...))
	if idx == -1 {
		if bytes.HasPrefix(rest, frontmatterDelimiter) {
			return []byte{}, skipNewline(rest[len(frontmatterDelimiter):]), nil
		}
		return nil, nil, fmt.Errorf("workflow file missing closing --- frontmatter delimiter")
	}

	frontmatter := rest[:idx]
	after := rest[idx+1+len(frontmatterDelimiter):]
	body := bytes.TrimLeft(skipNewline(after), "\n")

	return frontmatter, body, nil
}

func skipNewline(data []byte) []byte {
	if len(data) > 0 && data[0] == '\r' {
		data = data[1:]
	}
	if len(data) > 0 && data[0] == '\n' {
		data = data[1:]
	}
	return data
}

// applyCodexCompat maps a codex block to agent config when no agent block is set.
func applyCodexCompat(cfg *Config) {
	if cfg.Codex == nil || cfg.Agent.Command != "" {
		return
	}

	cfg.Agent.Kind = "codex"
	cfg.Agent.Command = cfg.Codex.Command

	if cfg.Agent.Config == nil {
		cfg.Agent.Config = make(map[string]any)
	}

	if cfg.Codex.ApprovalPolicy != "" {
		cfg.Agent.Config["approval_policy"] = cfg.Codex.ApprovalPolicy
	}
	if cfg.Codex.ThreadSandbox != "" {
		cfg.Agent.Config["thread_sandbox"] = cfg.Codex.ThreadSandbox
	}
	if cfg.Codex.TurnSandboxPolicy != nil {
		cfg.Agent.Config["turn_sandbox_policy"] = cfg.Codex.TurnSandboxPolicy
	}
	if cfg.Codex.TurnTimeoutMs != 0 {
		cfg.Agent.Config["turn_timeout_ms"] = cfg.Codex.TurnTimeoutMs
	}
	if cfg.Codex.StallTimeoutMs != 0 {
		cfg.Agent.Config["stall_timeout_ms"] = cfg.Codex.StallTimeoutMs
	}
}
