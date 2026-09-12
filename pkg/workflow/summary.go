package workflow

import (
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

type SummaryField struct {
	Label string
	Path  string
}

type RunSummary struct {
	Cells  []string
	Status string
}

func (w *Workflows) SetRunSchema(name string, fields []SummaryField) error {
	if strings.TrimSpace(name) == "" || len(fields) < 2 || len(fields) > 3 {
		return errors.New("run summaries require a workflow name and two or three fields")
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if strings.TrimSpace(field.Label) == "" || seen[field.Label] || (field.Path != "output" && !strings.HasPrefix(field.Path, "output.") && !strings.HasPrefix(field.Path, "input.")) {
			return errors.New("summary fields require unique labels and input or output paths")
		}
		seen[field.Label] = true
	}
	if w.runSchemas == nil {
		w.runSchemas = make(map[string][]SummaryField)
	}
	w.runSchemas[name] = append([]SummaryField(nil), fields...)
	return nil
}

func (w *Workflows) RunSchema(name string) []SummaryField {
	if fields, ok := w.runSchemas[name]; ok {
		return append([]SummaryField(nil), fields...)
	}
	return []SummaryField{{Label: "Output", Path: "output"}}
}

func SummarizeRun(run dbos.WorkflowStatus, fields []SummaryField) RunSummary {
	summary := RunSummary{Status: auditStatus(run.Status)}
	values := map[string]any{"input": summaryValue(run.Input), "output": summaryValue(run.Output)}
	for _, field := range fields {
		var value any = values
		for _, key := range strings.Split(field.Path, ".") {
			object, ok := value.(map[string]any)
			if !ok {
				value = nil
				break
			}
			value = object[key]
		}
		cell := "—"
		if value != nil {
			if list, ok := value.([]any); ok {
				value = len(list)
			}
			cell = strings.Join(strings.Fields(auditText(value)), " ")
			if cell == "" {
				cell = "—"
			}
			if chars := []rune(cell); len(chars) > 160 {
				cell = string(chars[:157]) + "…"
			}
		}
		summary.Cells = append(summary.Cells, cell)
	}
	return summary
}

func summaryValue(value any) any {
	if data, err := json.Marshal(value); err == nil {
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		if decoder.Decode(&decoded) == nil {
			value = decoded
		}
	}
	for depth := 0; depth < 8; depth++ {
		raw, ok := value.(string)
		if !ok || !json.Valid([]byte(raw)) {
			break
		}
		value = auditValue(raw)
	}
	return value
}

func (w *Workflows) RunningCount() (int, error) {
	var count int
	err := w.conn.QueryRowContext(w.ctx, "SELECT COUNT(*) FROM workflow_status WHERE status = ?", dbos.WorkflowStatusPending).Scan(&count)
	return count, errors.Wrap(err, "count running workflows")
}

func (w *Workflows) RunsByID(ids []string) ([]dbos.WorkflowStatus, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return dbos.ListWorkflows(w.ctx, dbos.WithWorkflowIDs(ids), dbos.WithSortDesc(), dbos.WithLoadInput(true), dbos.WithLoadOutput(true))
}
