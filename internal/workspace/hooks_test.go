package workspace

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunHook_Success(t *testing.T) {
	dir := t.TempDir()

	err := RunHook(context.Background(), "test", "echo hello > output.txt", dir, 5*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("unexpected output: %q", content)
	}
}

func TestRunHook_Failure(t *testing.T) {
	dir := t.TempDir()

	err := RunHook(context.Background(), "test", "exit 1", dir, 5*time.Second, slog.Default())
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
}

func TestRunHook_Timeout(t *testing.T) {
	dir := t.TempDir()

	err := RunHook(context.Background(), "test", "sleep 10", dir, 100*time.Millisecond, slog.Default())
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestRunHook_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()

	err := RunHook(context.Background(), "test", "pwd > cwd.txt", dir, 5*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "cwd.txt"))
	if err != nil {
		t.Fatalf("cwd file not created: %v", err)
	}

	// The output should be the temp dir path
	got := string(content)
	if got[:len(got)-1] != dir { // trim trailing newline
		t.Errorf("working directory = %q, want %q", got, dir)
	}
}
