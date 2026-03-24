package orchestrator

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ryanjdillon/symphony/internal/agent"
	"github.com/ryanjdillon/symphony/internal/config"
	"github.com/ryanjdillon/symphony/internal/telemetry"
	"github.com/ryanjdillon/symphony/internal/tracker/linear"
	"github.com/ryanjdillon/symphony/internal/worker"
	"github.com/ryanjdillon/symphony/internal/workspace"
)

// OrchestratorFactory holds the dependencies needed to construct an orchestrator
// from a workflow config.
type OrchestratorFactory struct {
	NewRunner     func(cfg *config.Config, logger *slog.Logger) (agent.Runner, error)
	NewSSHRunner  func(cfg *config.Config, logger *slog.Logger) *agent.SSHRunner
	NewHostMgr    func(cfg *config.Config, logger *slog.Logger) *worker.HostManager
	BuildTools    func(cfg *config.Config, logger *slog.Logger) []agent.ToolHandler
	Metrics       *telemetry.Metrics
	OnStateChange StateChangeFunc
	Logger        *slog.Logger
}

// MultiOrchestrator manages multiple workflow orchestrators.
type MultiOrchestrator struct {
	mu            sync.RWMutex
	orchestrators map[string]*managedOrch // keyed by workflow name
	factory       *OrchestratorFactory
	logger        *slog.Logger
}

type managedOrch struct {
	orch   *Orchestrator
	cancel context.CancelFunc
}

// NewMulti creates a new MultiOrchestrator.
func NewMulti(factory *OrchestratorFactory) *MultiOrchestrator {
	return &MultiOrchestrator{
		orchestrators: make(map[string]*managedOrch),
		factory:       factory,
		logger:        factory.Logger,
	}
}

// WorkflowName derives a workflow name from a file path.
func WorkflowName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// AddWorkflow loads a workflow file, creates an orchestrator, and starts it.
func (m *MultiOrchestrator) AddWorkflow(ctx context.Context, path string) error {
	name := WorkflowName(path)

	cfg, err := config.LoadWorkflow(path)
	if err != nil {
		return err
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing orchestrator for this name if present
	if existing, ok := m.orchestrators[name]; ok {
		existing.cancel()
		delete(m.orchestrators, name)
	}

	orch, err := m.buildOrchestrator(name, cfg)
	if err != nil {
		return err
	}

	orchCtx, cancel := context.WithCancel(ctx)
	m.orchestrators[name] = &managedOrch{orch: orch, cancel: cancel}

	go func() {
		m.logger.Info("workflow started", "workflow", name, "path", path)
		if err := orch.Run(orchCtx); err != nil && err != context.Canceled {
			m.logger.Error("workflow error", "workflow", name, "error", err)
		}
	}()

	return nil
}

// RemoveWorkflow stops and removes a workflow orchestrator.
func (m *MultiOrchestrator) RemoveWorkflow(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if managed, ok := m.orchestrators[name]; ok {
		m.logger.Info("stopping workflow", "workflow", name)
		managed.cancel()
		delete(m.orchestrators, name)
	}
}

// Snapshot returns an aggregated snapshot across all orchestrators.
func (m *MultiOrchestrator) Snapshot() StateSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var merged StateSnapshot
	for _, managed := range m.orchestrators {
		snap := managed.orch.Snapshot()
		merged.Running = append(merged.Running, snap.Running...)
		merged.Retrying = append(merged.Retrying, snap.Retrying...)
		merged.TokenTotals.Input += snap.TokenTotals.Input
		merged.TokenTotals.Output += snap.TokenTotals.Output
		merged.TokenTotals.Total += snap.TokenTotals.Total
		if snap.RuntimeSecs > merged.RuntimeSecs {
			merged.RuntimeSecs = snap.RuntimeSecs
		}
	}
	return merged
}

// TriggerRefresh triggers an immediate poll on all orchestrators.
func (m *MultiOrchestrator) TriggerRefresh(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, managed := range m.orchestrators {
		managed.orch.TriggerRefresh(ctx)
	}
}

// StopAll gracefully stops all orchestrators.
func (m *MultiOrchestrator) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, managed := range m.orchestrators {
		m.logger.Info("stopping workflow", "workflow", name)
		managed.cancel()
	}
	m.orchestrators = make(map[string]*managedOrch)
}

// Names returns the names of all running workflows.
func (m *MultiOrchestrator) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.orchestrators))
	for name := range m.orchestrators {
		names = append(names, name)
	}
	return names
}

func (m *MultiOrchestrator) buildOrchestrator(name string, cfg *config.Config) (*Orchestrator, error) {
	f := m.factory
	logger := f.Logger

	apiKey := config.ResolveEnvVars(cfg.Tracker.APIKey)
	trk := linear.NewClient(apiKey, cfg.Tracker.ProjectSlug, cfg.Tracker.ActiveStates, cfg.Tracker.TerminalStates, logger)
	wsMgr := workspace.NewManager(cfg.Workspace.Root, cfg.Workspace.Hooks, logger)

	runner, err := f.NewRunner(cfg, logger)
	if err != nil {
		return nil, err
	}

	var sshRunner *agent.SSHRunner
	var hostMgr *worker.HostManager
	if f.NewSSHRunner != nil && len(cfg.Worker.SSHHosts) > 0 {
		sshRunner = f.NewSSHRunner(cfg, logger)
		hostMgr = f.NewHostMgr(cfg, logger)
	}

	tools := f.BuildTools(cfg, logger)

	return New(name, cfg, trk, wsMgr, runner, sshRunner, hostMgr, tools, f.Metrics, logger, f.OnStateChange), nil
}
