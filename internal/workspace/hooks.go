package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// RunHook executes a shell script in the given working directory with a timeout.
func RunHook(ctx context.Context, name, script, workdir string, timeout time.Duration, logger *slog.Logger) error {
	logger.Info("running hook", "hook", name, "workdir", workdir)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		logger.Error("hook timed out", "hook", name, "timeout", timeout)
		return fmt.Errorf("hook %s timed out after %s", name, timeout)
	}
	if err != nil {
		logger.Error("hook failed", "hook", name, "error", err, "output", string(output))
		return fmt.Errorf("hook %s failed: %w (output: %s)", name, err, truncateOutput(output))
	}

	logger.Info("hook completed", "hook", name)
	return nil
}

func truncateOutput(output []byte) string {
	const maxLen = 512
	if len(output) <= maxLen {
		return string(output)
	}
	return string(output[:maxLen]) + "..."
}
