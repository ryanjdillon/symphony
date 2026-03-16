package template

import (
	"strings"
	"testing"
	"time"

	"github.com/ryanjdillon/symphony/internal/tracker"
)

func TestRender(t *testing.T) {
	issue := &tracker.Issue{
		Identifier:  "SYM-42",
		Title:       "Add authentication",
		Description: "Implement OAuth2 flow",
		CreatedAt:   time.Now(),
	}

	tmpl := "Working on {{ .Issue.Identifier }}: {{ .Issue.Title }}\n{{ .Issue.Description }}"

	got, err := Render(tmpl, issue, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "SYM-42") {
		t.Errorf("output missing identifier: %s", got)
	}
	if !strings.Contains(got, "Add authentication") {
		t.Errorf("output missing title: %s", got)
	}
}

func TestRender_DefaultTemplate(t *testing.T) {
	issue := &tracker.Issue{
		Identifier:  "SYM-1",
		Title:       "Test",
		Description: "Description here",
	}

	got, err := Render("", issue, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "SYM-1") {
		t.Errorf("default template should include identifier: %s", got)
	}
}

func TestRender_StrictMode(t *testing.T) {
	issue := &tracker.Issue{Identifier: "SYM-1", Title: "Test"}
	_, err := Render("{{ .NonExistent }}", issue, nil)
	if err == nil {
		t.Error("expected error for unknown variable in strict mode")
	}
}

func TestRender_WithAttempt(t *testing.T) {
	issue := &tracker.Issue{Identifier: "SYM-1", Title: "Test"}
	attempt := 3
	tmpl := "Attempt {{ .Attempt }}: {{ .Issue.Identifier }}"

	got, err := Render(tmpl, issue, &attempt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "3") {
		t.Errorf("output should contain attempt number: %s", got)
	}
}
