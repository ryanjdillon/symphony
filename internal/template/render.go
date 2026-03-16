package template

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/ryanjdillon/symphony/internal/tracker"
)

const defaultTemplate = `You are working on {{ .Issue.Identifier }}: {{ .Issue.Title }}

{{ .Issue.Description }}`

type renderContext struct {
	Issue   tracker.Issue
	Attempt *int
}

// Render executes the prompt template with the given issue and attempt context.
// Uses strict mode: unknown variables cause an error.
// If tmpl is empty, a default template is used.
func Render(tmpl string, issue *tracker.Issue, attempt *int) (string, error) {
	if tmpl == "" {
		tmpl = defaultTemplate
	}

	t, err := template.New("prompt").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing prompt template: %w", err)
	}

	ctx := renderContext{
		Issue:   *issue,
		Attempt: attempt,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing prompt template: %w", err)
	}

	return buf.String(), nil
}
