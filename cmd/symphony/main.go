package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryanjdillon/symphony/internal/agent/claudecode"
	"github.com/ryanjdillon/symphony/internal/agent/codex"
	"github.com/ryanjdillon/symphony/internal/config"
	"github.com/ryanjdillon/symphony/internal/orchestrator"
	"github.com/ryanjdillon/symphony/internal/status"
	"github.com/ryanjdillon/symphony/internal/tracker/linear"
	"github.com/ryanjdillon/symphony/internal/workspace"

	"github.com/ryanjdillon/symphony/internal/agent"
)

func main() {
	workflowPath := flag.String("workflow", "WORKFLOW.md", "path to WORKFLOW.md")
	port := flag.Int("port", 0, "HTTP status server port (0 = disabled)")
	jsonLogs := flag.Bool("json-logs", false, "use JSON log format")
	flag.Parse()

	logger := status.NewLogger(*jsonLogs)
	slog.SetDefault(logger)

	cfg, err := config.LoadWorkflow(*workflowPath)
	if err != nil {
		logger.Error("failed to load workflow", "path", *workflowPath, "error", err)
		os.Exit(1)
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Override server port from CLI flag
	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Initialize tracker
	trk := linear.NewClient(
		config.ResolveEnvVars(cfg.Tracker.APIKey),
		cfg.Tracker.ProjectSlug,
		cfg.Tracker.ActiveStates,
		cfg.Tracker.TerminalStates,
		logger,
	)

	// Initialize workspace manager
	wsMgr := workspace.NewManager(
		cfg.Workspace.Root,
		cfg.Workspace.Hooks,
		logger,
	)

	// Initialize agent runner
	runner, err := newRunner(cfg, logger)
	if err != nil {
		logger.Error("failed to create agent runner", "error", err)
		os.Exit(1)
	}

	// Set up state change callback for WebSocket broadcasting
	var srv *status.Server
	onStateChange := func(snap orchestrator.StateSnapshot) {
		if srv != nil {
			srv.Hub().Broadcast(snap)
		}
	}

	// Initialize orchestrator
	orch := orchestrator.New(cfg, trk, wsMgr, runner, logger, onStateChange)

	// Start file watcher for hot reload
	stopWatch, err := config.WatchWorkflow(*workflowPath, func(newCfg *config.Config) {
		newCfg.ApplyDefaults()
		if err := newCfg.Validate(); err != nil {
			logger.Warn("reloaded config invalid, keeping current", "error", err)
			return
		}
		orch.UpdateConfig(newCfg)
	}, logger)
	if err != nil {
		logger.Error("failed to start config watcher", "error", err)
		os.Exit(1)
	}
	defer stopWatch()

	// Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start status server if port configured
	if cfg.Server.Port > 0 {
		srv = status.NewServer(orch.Snapshot, func() { orch.TriggerRefresh(ctx) }, logger)
		addr := fmt.Sprintf("127.0.0.1:%d", cfg.Server.Port)
		go func() {
			if err := srv.Start(addr); err != nil {
				logger.Error("status server failed", "error", err)
			}
		}()
	}

	// Run orchestrator (blocks until context cancelled)
	if err := orch.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("orchestrator error", "error", err)
		os.Exit(1)
	}

	logger.Info("symphony stopped")
}

func newRunner(cfg *config.Config, logger *slog.Logger) (agent.Runner, error) {
	command := config.ResolveEnvVars(cfg.Agent.Command)
	kind := cfg.Agent.Kind

	switch kind {
	case "claude-code":
		return claudecode.NewRunner(command, logger), nil
	case "codex":
		return codex.NewRunner(command, logger), nil
	default:
		return nil, fmt.Errorf("unsupported agent kind: %s", kind)
	}
}
