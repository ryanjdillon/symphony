package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanjdillon/symphony/internal/config"
)

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SYM-123", "SYM-123"},
		{"SYM/123", "SYM_123"},
		{"hello world!", "hello_world_"},
		{"a.b.c", "a_b_c"},
		{"valid_key-123", "valid_key-123"},
		{"../../etc/passwd", "______etc_passwd"},
	}

	for _, tt := range tests {
		got := SanitizeKey(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEnsureWorkspace_CreatesNew(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, config.HooksConfig{}, slog.Default())

	path, created, err := mgr.EnsureWorkspace("SYM-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new workspace")
	}
	if path != filepath.Join(root, "SYM-1") {
		t.Errorf("path = %q, want %q", path, filepath.Join(root, "SYM-1"))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("workspace dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("workspace should be a directory")
	}
}

func TestEnsureWorkspace_ExistingReturnsNotCreated(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, config.HooksConfig{}, slog.Default())

	mgr.EnsureWorkspace("SYM-1")

	_, created, err := mgr.EnsureWorkspace("SYM-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false for existing workspace")
	}
}

func TestRemoveWorkspace(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, config.HooksConfig{}, slog.Default())

	mgr.EnsureWorkspace("SYM-1")

	if err := mgr.RemoveWorkspace("SYM-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "SYM-1")); !os.IsNotExist(err) {
		t.Error("workspace should be removed")
	}
}

func TestCleanupTerminal(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, config.HooksConfig{}, slog.Default())

	mgr.EnsureWorkspace("SYM-1")
	mgr.EnsureWorkspace("SYM-2")
	mgr.EnsureWorkspace("SYM-3")

	if err := mgr.CleanupTerminal([]string{"SYM-1", "SYM-3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SYM-1 and SYM-3 should be gone
	if _, err := os.Stat(filepath.Join(root, "SYM-1")); !os.IsNotExist(err) {
		t.Error("SYM-1 should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "SYM-3")); !os.IsNotExist(err) {
		t.Error("SYM-3 should be removed")
	}
	// SYM-2 should still exist
	if _, err := os.Stat(filepath.Join(root, "SYM-2")); err != nil {
		t.Error("SYM-2 should still exist")
	}
}

func TestWorkspacePath_Sanitized(t *testing.T) {
	root := "/tmp/workspaces"
	mgr := NewManager(root, config.HooksConfig{}, slog.Default())

	got := mgr.WorkspacePath("SYM/123")
	want := filepath.Join(root, "SYM_123")
	if got != want {
		t.Errorf("WorkspacePath(SYM/123) = %q, want %q", got, want)
	}
}
