package worker

import (
	"log/slog"
	"testing"
)

func newTestManager(hosts []string, maxPerHost int) *HostManager {
	return NewHostManager(hosts, maxPerHost, slog.Default())
}

func TestSelectHost_LeastLoaded(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 5)

	// First selection: both have 0 running, should pick first (host-a)
	h1, err := mgr.SelectHost()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1 != "host-a" {
		t.Errorf("expected host-a, got %s", h1)
	}

	// Second selection: host-a has 1 running, host-b has 0 — should pick host-b
	h2, err := mgr.SelectHost()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h2 != "host-b" {
		t.Errorf("expected host-b (least loaded), got %s", h2)
	}
}

func TestSelectHost_RespectsMaxPerHost(t *testing.T) {
	mgr := newTestManager([]string{"host-a"}, 2)

	_, _ = mgr.SelectHost() // running=1
	_, _ = mgr.SelectHost() // running=2

	// Third selection: at capacity
	_, err := mgr.SelectHost()
	if err == nil {
		t.Error("expected error when all hosts at capacity")
	}
}

func TestSelectHost_SkipsUnhealthy(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 5)

	mgr.MarkUnhealthy("host-a", "connection refused")

	host, err := mgr.SelectHost()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "host-b" {
		t.Errorf("expected host-b (host-a unhealthy), got %s", host)
	}
}

func TestReleaseHost(t *testing.T) {
	mgr := newTestManager([]string{"host-a"}, 1)

	_, _ = mgr.SelectHost() // running=1

	// At capacity
	_, err := mgr.SelectHost()
	if err == nil {
		t.Fatal("expected capacity error")
	}

	// Release and retry
	mgr.ReleaseHost("host-a")

	host, err := mgr.SelectHost()
	if err != nil {
		t.Fatalf("unexpected error after release: %v", err)
	}
	if host != "host-a" {
		t.Errorf("expected host-a, got %s", host)
	}
}

func TestMarkUnhealthy(t *testing.T) {
	mgr := newTestManager([]string{"host-a"}, 5)

	mgr.MarkUnhealthy("host-a", "timeout")

	snap := mgr.Snapshot()
	if snap[0].Healthy {
		t.Error("expected unhealthy after marking")
	}
	if snap[0].LastError != "timeout" {
		t.Errorf("last_error = %q, want %q", snap[0].LastError, "timeout")
	}
}

func TestHostForSession_PrefersExisting(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 5)

	host, err := mgr.HostForSession("host-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "host-b" {
		t.Errorf("expected preferred host-b, got %s", host)
	}
}

func TestHostForSession_FallsBackWhenPreferredUnhealthy(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 5)

	mgr.MarkUnhealthy("host-a", "down")

	host, err := mgr.HostForSession("host-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "host-b" {
		t.Errorf("expected fallback to host-b, got %s", host)
	}
}

func TestHostForSession_FallsBackWhenPreferredFull(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 1)

	_, _ = mgr.SelectHost() // host-a running=1

	host, err := mgr.HostForSession("host-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "host-b" {
		t.Errorf("expected fallback to host-b, got %s", host)
	}
}

func TestSnapshot(t *testing.T) {
	mgr := newTestManager([]string{"host-a", "host-b"}, 5)

	_, _ = mgr.SelectHost()
	mgr.MarkUnhealthy("host-b", "unreachable")

	snap := mgr.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 hosts in snapshot, got %d", len(snap))
	}

	if snap[0].Address != "host-a" || !snap[0].Healthy || snap[0].Running != 1 {
		t.Errorf("host-a snapshot: %+v", snap[0])
	}
	if snap[1].Address != "host-b" || snap[1].Healthy || snap[1].LastError != "unreachable" {
		t.Errorf("host-b snapshot: %+v", snap[1])
	}
}

func TestEnabled(t *testing.T) {
	mgr := newTestManager([]string{"host-a"}, 5)
	if !mgr.Enabled() {
		t.Error("expected enabled with hosts")
	}

	empty := newTestManager(nil, 5)
	if empty.Enabled() {
		t.Error("expected disabled with no hosts")
	}
}

func TestSelectHost_NoHosts(t *testing.T) {
	mgr := newTestManager(nil, 5)
	_, err := mgr.SelectHost()
	if err == nil {
		t.Error("expected error with no hosts")
	}
}

func TestReleaseHost_NeverNegative(t *testing.T) {
	mgr := newTestManager([]string{"host-a"}, 5)

	// Release without selecting — running should stay at 0
	mgr.ReleaseHost("host-a")

	snap := mgr.Snapshot()
	if snap[0].Running != 0 {
		t.Errorf("running = %d, want 0", snap[0].Running)
	}
}
