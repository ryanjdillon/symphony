package orchestrator

import (
	"testing"
)

func TestWorkflowName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"WORKFLOW.md", "WORKFLOW"},
		{"/etc/symphony/WORKFLOW.md", "WORKFLOW"},
		{"workflows/backend.md", "backend"},
		{"frontend-workflow.md", "frontend-workflow"},
		{"path/to/my-project.yaml.md", "my-project.yaml"},
	}

	for _, tt := range tests {
		got := WorkflowName(tt.path)
		if got != tt.want {
			t.Errorf("WorkflowName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
