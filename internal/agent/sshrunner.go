package agent

import (
	"context"
	"fmt"
	"log/slog"
)

// SSHRunner wraps an AppServerRunner to execute agent commands on a remote host
// via SSH. The stdio JSON protocol is piped over the SSH connection.
type SSHRunner struct {
	baseCommand string
	logger      *slog.Logger
}

// NewSSHRunner creates a runner that launches agents over SSH.
// The baseCommand is the agent command (e.g., "claude-code app-server").
func NewSSHRunner(baseCommand string, logger *slog.Logger) *SSHRunner {
	return &SSHRunner{
		baseCommand: baseCommand,
		logger:      logger,
	}
}

func (r *SSHRunner) Name() string { return "ssh" }

// StartOnHost launches the agent on the given SSH host.
// The command becomes: ssh -o StrictHostKeyChecking=accept-new <host> <baseCommand>
// Workspace root is interpreted on the remote host.
func (r *SSHRunner) StartOnHost(ctx context.Context, host string, opts *StartOpts) (Session, error) {
	if host == "" {
		return nil, fmt.Errorf("ssh host is empty")
	}

	sshCommand := fmt.Sprintf("ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes %s %s", host, r.baseCommand)
	delegate := NewAppServerRunner("ssh:"+host, sshCommand, r.logger)
	return delegate.Start(ctx, opts)
}
