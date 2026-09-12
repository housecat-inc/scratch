package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

type AuditRun struct {
	Attempts    int         `json:"attempts"`
	CompletedAt time.Time   `json:"completed_at"`
	CreatedAt   time.Time   `json:"created_at"`
	Error       string      `json:"error,omitempty"`
	ID          string      `json:"id"`
	Input       any         `json:"input"`
	Name        string      `json:"name"`
	Output      any         `json:"output"`
	ParentID    string      `json:"parent_id,omitempty"`
	StartedAt   time.Time   `json:"started_at"`
	Status      string      `json:"status"`
	Steps       []AuditStep `json:"steps"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type AuditStep struct {
	ChildID     string    `json:"child_id,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Output      any       `json:"output"`
	StartedAt   time.Time `json:"started_at"`
}

func (w *Workflows) Audit(id string) (*AuditRun, error) {
	runs, err := dbos.ListWorkflows(w.ctx, dbos.WithWorkflowIDs([]string{id}), dbos.WithLoadInput(true), dbos.WithLoadOutput(true))
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	run := runs[0]
	steps, err := dbos.GetWorkflowSteps(w.ctx, id, dbos.WithStepsLoadOutput(true))
	if err != nil {
		return nil, err
	}
	audit := &AuditRun{
		Attempts: run.Attempts, CompletedAt: run.CompletedAt, CreatedAt: run.CreatedAt,
		Error: auditError(run.Error), ID: run.ID, Input: auditValue(run.Input), Name: run.Name,
		Output: auditValue(run.Output), ParentID: run.ParentWorkflowID, StartedAt: run.StartedAt,
		Status: string(run.Status), Steps: make([]AuditStep, 0, len(steps)), UpdatedAt: run.UpdatedAt,
	}
	for _, step := range steps {
		audit.Steps = append(audit.Steps, AuditStep{
			ChildID: step.ChildWorkflowID, CompletedAt: step.CompletedAt, Error: auditError(step.Error),
			ID: step.StepID, Name: step.StepName, Output: auditValue(step.Output), StartedAt: step.StartedAt,
		})
	}
	return audit, nil
}

func auditError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func auditValue(value any) any {
	encoded, ok := value.(string)
	if !ok || !json.Valid([]byte(encoded)) {
		return value
	}
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return value
	}
	return decoded
}

func auditLabel(name string) string {
	var result []rune
	previous := rune(0)
	for _, char := range name {
		if char == '_' || char == '-' || char == '/' || char == '.' {
			char = ' '
		} else if unicode.IsUpper(char) && (unicode.IsLower(previous) || unicode.IsDigit(previous)) {
			result = append(result, ' ')
		}
		result = append(result, char)
		previous = char
	}
	label := []rune(strings.Join(strings.Fields(string(result)), " "))
	if len(label) > 0 {
		label[0] = unicode.ToUpper(label[0])
	}
	return string(label)
}

func auditStatus(value any) string {
	status := fmt.Sprint(value)
	switch status {
	case "CANCELLED":
		return "Cancelled"
	case "DELAYED":
		return "Scheduled for later"
	case "ENQUEUED":
		return "Waiting to start"
	case "ERROR":
		return "Failed"
	case "MAX_RECOVERY_ATTEMPTS_EXCEEDED":
		return "Stopped after exhausting recovery attempts"
	case "PENDING":
		return "In progress"
	case "SUCCESS":
		return "Completed successfully"
	default:
		return auditLabel(status)
	}
}

func auditText(value any) string {
	return auditTextDepth(value, 0)
}

func auditTextDepth(value any, depth int) string {
	if depth > 32 {
		return "Further nested details omitted."
	}
	switch value := value.(type) {
	case nil:
		return "None"
	case string:
		if decoded := auditValue(value); decoded != value {
			return auditTextDepth(decoded, depth+1)
		}
		lines := strings.Split(value, "\n")
		for i, line := range lines {
			if json.Valid([]byte(line)) {
				lines[i] = auditTextDepth(auditValue(line), depth+1)
			}
		}
		return strings.Join(lines, "\n")
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			text := auditTextDepth(value[key], depth+1)
			lines = append(lines, auditLabel(key)+": "+strings.ReplaceAll(text, "\n", "\n  "))
		}
		if len(lines) == 0 {
			return "None"
		}
		return strings.Join(lines, "\n")
	case []any:
		if len(value) == 0 {
			return "None"
		}
		lines := make([]string, 0, len(value))
		for i, item := range value {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, strings.ReplaceAll(auditTextDepth(item, depth+1), "\n", "\n   ")))
		}
		return strings.Join(lines, "\n")
	case bool:
		if value {
			return "Yes"
		}
		return "No"
	default:
		return fmt.Sprint(value)
	}
}

func (w *Workflows) RegisterAudit(mux *http.ServeMux) {
	mux.HandleFunc("GET /workflows/audit", w.handleAudit)
}

func (w *Workflows) handleAudit(out http.ResponseWriter, request *http.Request) {
	out.Header().Set("Cache-Control", "no-store")
	data := struct {
		Next int
		Run  *AuditRun
		Runs []dbos.WorkflowStatus
	}{}
	if id := request.URL.Query().Get("run"); id != "" {
		run, err := w.Audit(id)
		if err != nil {
			http.Error(out, "Unable to read workflow audit.", http.StatusInternalServerError)
			return
		}
		if run == nil {
			http.NotFound(out, request)
			return
		}
		data.Run = run
	} else {
		offset := 0
		if raw := request.URL.Query().Get("offset"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 || value > 1000000000 {
				http.Error(out, "Invalid offset.", http.StatusBadRequest)
				return
			}
			offset = value
		}
		runs, err := dbos.ListWorkflows(w.ctx, dbos.WithLimit(51), dbos.WithOffset(offset), dbos.WithSortDesc(), dbos.WithLoadInput(false), dbos.WithLoadOutput(false))
		if err != nil {
			http.Error(out, "Unable to read workflow history.", http.StatusInternalServerError)
			return
		}
		if len(runs) > 50 {
			data.Next = offset + 50
			runs = runs[:50]
		}
		data.Runs = runs
	}
	var body bytes.Buffer
	if err := auditPage.Execute(&body, data); err != nil {
		http.Error(out, "Unable to render workflow audit.", http.StatusInternalServerError)
		return
	}
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	out.Write(body.Bytes())
}

var auditPage = template.Must(template.New("audit").Funcs(template.FuncMap{
	"label":  auditLabel,
	"text":   auditText,
	"status": auditStatus,
	"number": func(index int) int { return index + 1 },
	"name": func(value string) string {
		if strings.Contains(value, "/") {
			value = value[strings.LastIndex(value, ".")+1:]
		}
		return auditLabel(value)
	},
	"time": func(value time.Time) string {
		if value.IsZero() {
			return "Not recorded"
		}
		return value.UTC().Format("Jan 2, 2006 at 15:04:05 UTC")
	},
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Workflow audit · scratch</title>
<style>body{font:16px system-ui;max-width:1000px;margin:32px auto;padding:0 20px;color:#17202a;overflow-wrap:anywhere}a{color:#2850b8}pre{white-space:pre-wrap;background:#f4f5f7;padding:16px;border-radius:8px}article{border-bottom:1px solid #ddd;padding:16px 0}dt{font-weight:bold}dd{margin:0 0 12px}summary{cursor:pointer;padding:10px 0}</style></head><body>
<nav><a href="/inbox/workflows">Workflows</a> · <a href="/workflows/audit">All run audits</a></nav>
{{if .Run}}{{with .Run}}
<h1>{{name .Name}}</h1><p>{{status .Status}} · <code>{{.ID}}</code></p>
<dl><dt>Created</dt><dd>{{time .CreatedAt}}</dd><dt>Started</dt><dd>{{time .StartedAt}}</dd><dt>Completed</dt><dd>{{time .CompletedAt}}</dd><dt>Last updated</dt><dd>{{time .UpdatedAt}}</dd><dt>Execution attempts</dt><dd>{{.Attempts}}</dd></dl>
{{if .ParentID}}<p><a href="/workflows/audit?run={{.ParentID}}">Parent run</a></p>{{end}}
<h2>Run outcome</h2><p>{{status .Status}}</p>{{if .Error}}<pre>{{text .Error}}</pre>{{end}}{{if ne .Output nil}}<pre>{{text .Output}}</pre>{{end}}
<details><summary>Input</summary><pre>{{text .Input}}</pre></details>
<h2>Step-by-step audit log</h2>
{{range $index, $step := .Steps}}<article><h3>Step {{number $index}}: {{name .Name}}</h3><p>Started: {{time .StartedAt}}</p>{{if .Error}}<p>Failed: {{time .CompletedAt}}</p><pre>{{text .Error}}</pre>{{else if .CompletedAt.IsZero}}<p>Waiting for completion.</p>{{else}}<p>Completed: {{time .CompletedAt}}</p>{{end}}{{if ne .Output nil}}<pre>{{text .Output}}</pre>{{else if not .Error}}<p>No additional result was recorded.</p>{{end}}{{if .ChildID}}<a href="/workflows/audit?run={{.ChildID}}">View child run</a>{{end}}</article>{{else}}<p>No steps recorded yet.</p>{{end}}
<p>This log includes every recorded workflow step. Work inside a script or agent appears only when that step records it; detailed agent transcripts remain separate.</p>
{{end}}{{else}}<h1>Workflow audit history</h1><p>Every run is recorded automatically in the workflow database, including scheduled, manual, and chat-triggered runs. Select a run to inspect its outcome, inputs, and recorded steps.</p>
{{range .Runs}}<article><h2><a href="/workflows/audit?run={{.ID}}">{{name .Name}}</a></h2><p>{{status .Status}} · {{time .CreatedAt}}</p><code>{{.ID}}</code></article>{{else}}<p>No workflow runs yet.</p>{{end}}
{{if .Next}}<p><a href="/workflows/audit?offset={{.Next}}">Older runs</a></p>{{end}}{{end}}
</body></html>`))
