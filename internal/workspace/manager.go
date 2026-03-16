package workspace

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ryanjdillon/symphony/internal/config"
)

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Manager maps issue identifiers to filesystem workspace paths.
type Manager struct {
	root   string
	hooks  config.HooksConfig
	logger *slog.Logger
}

// NewManager creates a workspace manager rooted at the given directory.
func NewManager(root string, hooks config.HooksConfig, logger *slog.Logger) *Manager {
	return &Manager{
		root:   root,
		hooks:  hooks,
		logger: logger,
	}
}

// SanitizeKey replaces characters not in [a-zA-Z0-9_-] with underscore.
func SanitizeKey(identifier string) string {
	return unsafeChars.ReplaceAllString(identifier, "_")
}

// WorkspacePath returns the absolute path for the given identifier.
func (m *Manager) WorkspacePath(identifier string) string {
	return filepath.Join(m.root, SanitizeKey(identifier))
}

func (m *Manager) validatePath(wsPath string) error {
	rel, err := filepath.Rel(m.root, wsPath)
	if err != nil {
		return fmt.Errorf("workspace path validation: %w", err)
	}
	if len(rel) >= 2 && rel[:2] == ".." {
		return fmt.Errorf("workspace path %q escapes root %q", wsPath, m.root)
	}
	return nil
}

// EnsureWorkspace creates the workspace directory if it doesn't exist.
// Returns the path, whether it was newly created, and any error.
func (m *Manager) EnsureWorkspace(identifier string) (wsPath string, created bool, err error) {
	wsPath = m.WorkspacePath(identifier)
	if err := m.validatePath(wsPath); err != nil {
		return "", false, err
	}

	info, statErr := os.Stat(wsPath)
	if statErr == nil && info.IsDir() {
		return wsPath, false, nil
	}

	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		return "", false, fmt.Errorf("creating workspace: %w", err)
	}

	m.logger.Info("workspace created", "identifier", identifier, "path", wsPath)
	return wsPath, true, nil
}

// RemoveWorkspace removes the workspace directory for the given identifier.
func (m *Manager) RemoveWorkspace(identifier string) error {
	wsPath := m.WorkspacePath(identifier)
	if err := m.validatePath(wsPath); err != nil {
		return err
	}

	if err := os.RemoveAll(wsPath); err != nil {
		return fmt.Errorf("removing workspace %q: %w", wsPath, err)
	}

	m.logger.Info("workspace removed", "identifier", identifier, "path", wsPath)
	return nil
}

// CleanupTerminal removes workspaces for all provided identifiers.
func (m *Manager) CleanupTerminal(identifiers []string) error {
	var errs []error
	for _, id := range identifiers {
		if err := m.RemoveWorkspace(id); err != nil {
			m.logger.Error("cleanup failed", "identifier", id, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup encountered %d error(s)", len(errs))
	}
	return nil
}
