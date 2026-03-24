package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanjdillon/symphony/internal/agent"
	"github.com/ryanjdillon/symphony/internal/agent/claudecode"
	"github.com/ryanjdillon/symphony/internal/agent/codex"
	"github.com/ryanjdillon/symphony/internal/agent/tools"
	"github.com/ryanjdillon/symphony/internal/config"
	"github.com/ryanjdillon/symphony/internal/orchestrator"
	"github.com/ryanjdillon/symphony/internal/status"
	"github.com/ryanjdillon/symphony/internal/telemetry"
	"github.com/ryanjdillon/symphony/internal/worker"
)

type workflowFlags []string

func (w *workflowFlags) String() string { return strings.Join(*w, ", ") }
func (w *workflowFlags) Set(value string) error {
	*w = append(*w, value)
	return nil
}

func main() {
	os.Exit(run())
}

func run() int {
	var workflows workflowFlags
	flag.Var(&workflows, "workflow", "path to WORKFLOW.md (can be specified multiple times)")
	workflowDir := flag.String("workflow-dir", "", "directory of workflow .md files")
	port := flag.Int("port", 0, "HTTP status server port (0 = disabled)")
	jsonLogs := flag.Bool("json-logs", false, "use JSON log format")
	flag.Parse()

	logger := status.NewLogger(*jsonLogs)
	slog.SetDefault(logger)

	// Default to single workflow if nothing specified
	if len(workflows) == 0 && *workflowDir == "" {
		workflows = []string{"WORKFLOW.md"}
	}

	// Initialize OpenTelemetry (best-effort: if OTEL endpoint not set, metrics are no-op)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	otelProvider, err := telemetry.Init(ctx)
	if err != nil {
		logger.Warn("OTEL initialization failed, metrics disabled", "error", err)
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := otelProvider.Shutdown(shutdownCtx); err != nil {
				logger.Warn("OTEL shutdown error", "error", err)
			}
		}()
	}

	metrics := telemetry.NewMetrics(logger)

	// Set up MultiOrchestrator
	var srv *status.Server
	multi := orchestrator.NewMulti(&orchestrator.OrchestratorFactory{
		NewRunner:    newRunner,
		NewSSHRunner: newSSHRunner,
		NewHostMgr:   newHostMgr,
		BuildTools:   buildTools,
		Metrics:      metrics,
		OnStateChange: func(snap *orchestrator.StateSnapshot) {
			if srv != nil {
				srv.Hub().Broadcast(snap)
			}
		},
		Logger: logger,
	})

	// Load explicit workflow files
	for _, path := range workflows {
		if err := multi.AddWorkflow(ctx, path); err != nil {
			logger.Error("failed to load workflow", "path", path, "error", err)
			return 1
		}
	}

	// Load workflow directory
	if *workflowDir != "" {
		entries, err := os.ReadDir(*workflowDir)
		if err != nil {
			logger.Error("failed to read workflow directory", "dir", *workflowDir, "error", err)
			return 1
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(*workflowDir, entry.Name())
			if err := multi.AddWorkflow(ctx, path); err != nil {
				logger.Error("failed to load workflow", "path", path, "error", err)
				return 1
			}
		}

		// Watch directory for new/removed workflow files
		go watchWorkflowDir(ctx, *workflowDir, multi, logger)
	}

	// Start status server if port configured
	if *port > 0 {
		srv = status.NewServer(multi.Snapshot, func() { multi.TriggerRefresh(ctx) }, logger)
		addr := fmt.Sprintf("127.0.0.1:%d", *port)
		go func() {
			if err := srv.Start(addr); err != nil {
				logger.Error("status server failed", "error", err)
			}
		}()
	}

	// Block until signal
	<-ctx.Done()
	logger.Info("shutting down")
	multi.StopAll()
	logger.Info("symphony stopped")
	return 0
}

func watchWorkflowDir(ctx context.Context, dir string, multi *orchestrator.MultiOrchestrator, logger *slog.Logger) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("failed to watch workflow directory", "error", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(dir); err != nil {
		logger.Error("failed to add workflow directory watch", "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}

			name := orchestrator.WorkflowName(event.Name)

			switch {
			case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
				logger.Info("workflow file changed, loading", "path", event.Name)
				if err := multi.AddWorkflow(ctx, event.Name); err != nil {
					logger.Error("failed to load workflow", "path", event.Name, "error", err)
				}
			case event.Op&fsnotify.Remove != 0:
				logger.Info("workflow file removed, stopping", "name", name)
				multi.RemoveWorkflow(name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("workflow directory watcher error", "error", err)
		}
	}
}

func newRunner(cfg *config.Config, logger *slog.Logger) (agent.Runner, error) {
	command := config.ResolveEnvVars(cfg.Agent.Command)
	switch cfg.Agent.Kind {
	case "claude-code":
		return claudecode.NewRunner(command, logger), nil
	case "codex":
		return codex.NewRunner(command, logger), nil
	default:
		return nil, fmt.Errorf("unsupported agent kind: %s", cfg.Agent.Kind)
	}
}

func newSSHRunner(cfg *config.Config, logger *slog.Logger) *agent.SSHRunner {
	return agent.NewSSHRunner(config.ResolveEnvVars(cfg.Agent.Command), logger)
}

func newHostMgr(cfg *config.Config, logger *slog.Logger) *worker.HostManager {
	return worker.NewHostManager(cfg.Worker.SSHHosts, cfg.Worker.MaxConcurrentAgentsPerHost, logger)
}

func buildTools(cfg *config.Config, logger *slog.Logger) []agent.ToolHandler {
	var t []agent.ToolHandler

	if cfg.Tracker.Kind == "linear" && cfg.Tracker.APIKey != "" {
		allowMutations := false
		if v, ok := cfg.Agent.Config["allow_linear_mutations"]; ok {
			if b, ok := v.(bool); ok {
				allowMutations = b
			}
		}
		apiKey := config.ResolveEnvVars(cfg.Tracker.APIKey)
		t = append(t, tools.NewLinearGraphQL(apiKey, allowMutations, logger))
		logger.Info("linear_graphql tool enabled", "allow_mutations", allowMutations)
	}

	return t
}
