package worker

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// HostState represents the health of an SSH host.
type HostState int

const (
	HostHealthy HostState = iota
	HostUnhealthy
)

// Host tracks the state and load of an SSH worker host.
type Host struct {
	Address        string
	State          HostState
	Running        int
	LastError      string
	UnhealthySince time.Time
}

const unhealthyCooldown = 60 * time.Second

// HostManager manages a pool of SSH hosts for worker dispatch.
type HostManager struct {
	mu         sync.RWMutex
	hosts      []*Host
	maxPerHost int
	logger     *slog.Logger
}

// NewHostManager creates a host manager with the given SSH destinations.
func NewHostManager(addresses []string, maxPerHost int, logger *slog.Logger) *HostManager {
	hosts := make([]*Host, len(addresses))
	for i, addr := range addresses {
		hosts[i] = &Host{Address: addr, State: HostHealthy}
	}
	if maxPerHost <= 0 {
		maxPerHost = 5
	}
	return &HostManager{
		hosts:      hosts,
		maxPerHost: maxPerHost,
		logger:     logger.With("component", "host_manager"),
	}
}

// SelectHost returns the least-loaded healthy host, or an error if none available.
func (m *HostManager) SelectHost() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var best *Host

	for _, h := range m.hosts {
		// Recover unhealthy hosts after cooldown
		if h.State == HostUnhealthy && now.Sub(h.UnhealthySince) > unhealthyCooldown {
			h.State = HostHealthy
			h.LastError = ""
			m.logger.Info("host recovered", "host", h.Address)
		}

		if h.State != HostHealthy {
			continue
		}
		if h.Running >= m.maxPerHost {
			continue
		}
		if best == nil || h.Running < best.Running {
			best = h
		}
	}

	if best == nil {
		return "", fmt.Errorf("no healthy hosts with available capacity")
	}

	best.Running++
	m.logger.Debug("host selected", "host", best.Address, "running", best.Running)
	return best.Address, nil
}

// ReleaseHost decrements the running count for a host.
func (m *HostManager) ReleaseHost(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, h := range m.hosts {
		if h.Address != address {
			continue
		}
		if h.Running > 0 {
			h.Running--
		}
		return
	}
}

// MarkUnhealthy marks a host as unhealthy after a connection failure.
func (m *HostManager) MarkUnhealthy(address, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, h := range m.hosts {
		if h.Address != address {
			continue
		}
		h.State = HostUnhealthy
		h.LastError = reason
		h.UnhealthySince = time.Now()
		m.logger.Warn("host marked unhealthy", "host", address, "reason", reason)
		return
	}
}

// HostForSession returns the address a session should use for continuation.
// If the preferred host is unhealthy or full, falls back to SelectHost.
func (m *HostManager) HostForSession(preferred string) (string, error) {
	if m.tryPreferred(preferred) {
		return preferred, nil
	}
	return m.SelectHost()
}

func (m *HostManager) tryPreferred(preferred string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, h := range m.hosts {
		if h.Address != preferred {
			continue
		}
		if h.State == HostHealthy && h.Running < m.maxPerHost {
			h.Running++
			return true
		}
		return false
	}
	return false
}

// Snapshot returns the current state of all hosts for the status API.
func (m *HostManager) Snapshot() []HostSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := make([]HostSnapshot, len(m.hosts))
	for i, h := range m.hosts {
		snap[i] = HostSnapshot{
			Address:   h.Address,
			Healthy:   h.State == HostHealthy,
			Running:   h.Running,
			LastError: h.LastError,
		}
	}
	return snap
}

// Enabled returns true if SSH hosts are configured.
func (m *HostManager) Enabled() bool {
	return len(m.hosts) > 0
}

// HostSnapshot is the read-only view of a host for the status API.
type HostSnapshot struct {
	Address   string `json:"address"`
	Healthy   bool   `json:"healthy"`
	Running   int    `json:"running"`
	LastError string `json:"last_error,omitempty"`
}
