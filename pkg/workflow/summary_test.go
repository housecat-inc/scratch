package workflow

import (
	"strings"
	"testing"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSummary(t *testing.T) {
	fields := []SummaryField{{Label: "Saved", Path: "output.saved"}, {Label: "Errors", Path: "output.errors"}, {Label: "Published", Path: "output.published"}}
	for _, tt := range []struct {
		name   string
		output any
		want   []string
	}{
		{name: "script JSON preserves zero and false", output: `{"saved":0,"errors":[],"published":false}`, want: []string{"0", "0", "No"}},
		{name: "structured output", output: map[string]any{"saved": 5, "errors": []string{"failed"}, "published": true}, want: []string{"5", "1", "Yes"}},
		{name: "persisted script JSON", output: `"{\"saved\":0,\"errors\":[],\"published\":false}"`, want: []string{"0", "0", "No"}},
		{name: "unfinished", want: []string{"—", "—", "—"}},
		{name: "legacy plain output", output: "old result", want: []string{"—", "—", "—"}},
		{name: "missing fields", output: `{"saved":2}`, want: []string{"2", "—", "—"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			summary := SummarizeRun(dbos.WorkflowStatus{Output: tt.output, Status: dbos.WorkflowStatusSuccess}, fields)
			a.Equal(tt.want, summary.Cells)
			a.Equal("Completed successfully", summary.Status)
		})
	}
	t.Run("nested input and compact text", func(t *testing.T) {
		a := assert.New(t)
		run := dbos.WorkflowStatus{Input: struct{ Prompt string }{Prompt: "A request"}, Output: map[string]any{"report": map[string]any{"title": strings.Repeat("界", 200)}}, Status: dbos.WorkflowStatusError}
		summary := SummarizeRun(run, []SummaryField{{Label: "Request", Path: "input.Prompt"}, {Label: "Report", Path: "output.report.title"}})
		a.Equal("A request", summary.Cells[0])
		a.LessOrEqual(len([]rune(summary.Cells[1])), 160)
		a.True(strings.HasSuffix(summary.Cells[1], "…"))
		a.Equal("Failed", summary.Status)
	})
}

func TestRunSchemaValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fields []SummaryField
		valid  bool
	}{
		{name: "two fields", fields: []SummaryField{{Label: "Saved", Path: "output.saved"}, {Label: "Errors", Path: "output.errors"}}, valid: true},
		{name: "empty"},
		{name: "too many", fields: make([]SummaryField, 4)},
		{name: "duplicate", fields: []SummaryField{{Label: "Saved", Path: "output.saved"}, {Label: "Saved", Path: "output.errors"}}},
		{name: "invalid source", fields: []SummaryField{{Label: "Saved", Path: "saved"}, {Label: "Errors", Path: "output.errors"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, r := assert.New(t), require.New(t)
			w := &Workflows{}
			err := w.SetRunSchema("fixture", tt.fields)
			if !tt.valid {
				r.Error(err)
				return
			}
			r.NoError(err)
			fields := w.RunSchema("fixture")
			a.Equal(tt.fields, fields)
			fields[0].Label = "changed"
			a.Equal("Saved", w.RunSchema("fixture")[0].Label)
		})
	}
}
