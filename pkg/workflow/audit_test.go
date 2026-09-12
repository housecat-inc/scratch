package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func auditFixture(ctx dbos.DBOSContext, fail bool) (string, error) {
	return dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		if fail {
			return "", errors.New("publish failed <script>alert(1)</script>")
		}
		return "Created task #42", nil
	}, dbos.WithStepName("publish"))
}

func TestAuditPersistence(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "error"
		}
		t.Run(name, func(t *testing.T) {
			a, r := assert.New(t), require.New(t)
			path := filepath.Join(t.TempDir(), "audit.db")
			w, err := New(path)
			r.NoError(err)
			dbos.RegisterWorkflow(w.Ctx(), auditFixture)
			r.NoError(w.Launch())
			handle, err := dbos.RunWorkflow(w.Ctx(), auditFixture, fail)
			r.NoError(err)
			_, err = handle.GetResult()
			if fail {
				r.Error(err)
			} else {
				r.NoError(err)
			}
			id := handle.GetWorkflowID()
			r.NoError(w.Close())
			w, err = New(path)
			r.NoError(err)
			dbos.RegisterWorkflow(w.Ctx(), auditFixture)
			r.NoError(w.Launch())
			t.Cleanup(func() { r.NoError(w.Close()) })
			audit, err := w.Audit(id)
			r.NoError(err)
			r.NotNil(audit)
			a.Equal(id, audit.ID)
			r.Len(audit.Steps, 1)
			a.Equal("publish", audit.Steps[0].Name)
			a.NotNil(audit.Input)
			a.False(audit.CreatedAt.IsZero())
			if fail {
				a.Equal("ERROR", audit.Status)
				a.Contains(audit.Error, "publish failed")
				a.Contains(audit.Steps[0].Error, "publish failed")
			} else {
				a.Equal("SUCCESS", audit.Status)
				a.Equal("Created task #42", audit.Output)
				a.Equal("Created task #42", audit.Steps[0].Output)
			}
			mux := http.NewServeMux()
			w.RegisterAudit(mux)
			for _, test := range []struct {
				name   string
				path   string
				status int
			}{
				{name: "legacy format remains readable", path: "/workflows/audit?run=" + url.QueryEscape(id) + "&format=json", status: 200},
				{name: "history", path: "/workflows/audit", status: 200},
				{name: "invalid offset", path: "/workflows/audit?offset=-1", status: 400},
				{name: "missing", path: "/workflows/audit?run=missing", status: 404},
				{name: "run", path: "/workflows/audit?run=" + url.QueryEscape(id), status: 200},
			} {
				t.Run(test.name, func(t *testing.T) {
					a, r := assert.New(t), require.New(t)
					response := httptest.NewRecorder()
					mux.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
					r.Equal(test.status, response.Code)
					a.Equal("no-store", response.Header().Get("Cache-Control"))
					if test.status == 200 {
						a.Contains(response.Header().Get("Content-Type"), "text/html")
						a.Empty(response.Header().Get("Content-Disposition"))
						a.Contains(response.Body.String(), id)
						a.NotContains(response.Body.String(), "Download audit JSON")
						a.NotContains(response.Body.String(), "<script>alert(1)</script>")
						if test.name != "history" {
							a.Contains(response.Body.String(), "Step 1: Publish")
							if fail {
								a.Contains(response.Body.String(), "Failed:")
							} else {
								a.Contains(response.Body.String(), "Created task #42")
							}
						}
					}
				})
			}
		})
	}
}

func TestAuditUnfinishedRun(t *testing.T) {
	a, r := assert.New(t), require.New(t)
	w, err := New(filepath.Join(t.TempDir(), "audit.db"))
	r.NoError(err)
	started := make(chan struct{})
	waiting := func(ctx dbos.DBOSContext, input string) (string, error) {
		if _, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
			return "Prepared report", nil
		}, dbos.WithStepName("prepare")); err != nil {
			return "", err
		}
		close(started)
		_, err := dbos.Sleep(ctx, time.Hour)
		return "", err
	}
	dbos.RegisterWorkflow(w.Ctx(), waiting)
	r.NoError(w.Launch())
	t.Cleanup(func() { r.NoError(w.Close()) })
	handle, err := dbos.RunWorkflow(w.Ctx(), waiting, "report")
	r.NoError(err)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("workflow did not start")
	}
	audit, err := w.Audit(handle.GetWorkflowID())
	r.NoError(err)
	a.Equal("PENDING", audit.Status)
	r.NotEmpty(audit.Steps)
	a.Equal("Prepared report", audit.Steps[0].Output)
	r.NoError(dbos.CancelWorkflow(w.Ctx(), handle.GetWorkflowID()))
	audit, err = w.Audit(handle.GetWorkflowID())
	r.NoError(err)
	a.Equal("CANCELLED", audit.Status)
	a.Equal("Prepared report", audit.Steps[0].Output)
}

func TestAuditValue(t *testing.T) {
	for _, test := range []struct {
		input string
		name  string
		want  any
	}{
		{input: `{"id":9007199254740993}`, name: "large identifier", want: map[string]any{"id": json.Number("9007199254740993")}},
		{input: `"{\"created\":1}"`, name: "string containing JSON", want: `{"created":1}`},
		{input: "failed to decode", name: "unavailable value", want: "failed to decode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(test.want, auditValue(test.input))
		})
	}
}

func TestAuditHistoryPagination(t *testing.T) {
	a, r := assert.New(t), require.New(t)
	w, err := New(filepath.Join(t.TempDir(), "audit.db"))
	r.NoError(err)
	r.NoError(w.Launch())
	t.Cleanup(func() { r.NoError(w.Close()) })
	for range 51 {
		_, err := w.Greet("Ada")
		r.NoError(err)
	}
	mux := http.NewServeMux()
	w.RegisterAudit(mux)
	first := httptest.NewRecorder()
	mux.ServeHTTP(first, httptest.NewRequest("GET", "/workflows/audit", nil))
	r.Equal(http.StatusOK, first.Code)
	a.Contains(first.Body.String(), "/workflows/audit?offset=50")
	older := httptest.NewRecorder()
	mux.ServeHTTP(older, httptest.NewRequest("GET", "/workflows/audit?offset=50", nil))
	r.Equal(http.StatusOK, older.Code)
	a.Contains(older.Body.String(), "Completed successfully")
	a.NotContains(older.Body.String(), "Older runs")
}

func TestAuditReadableResults(t *testing.T) {
	for _, test := range []struct {
		input any
		name  string
		want  string
	}{
		{input: map[string]any{"task_id": json.Number("9007199254740993"), "title": "Call Ada"}, name: "created record", want: "Task id: 9007199254740993\nTitle: Call Ada"},
		{input: "Collected metrics\n{\"snapshots_saved\":3,\"errors\":[]}\n", name: "process output", want: "Collected metrics\nErrors: None\nSnapshots saved: 3\n"},
		{input: []any{map[string]any{"name": "Ada"}, "Created task"}, name: "multiple results", want: "1. Name: Ada\n2. Created task"},
		{input: `{"accepted":true}`, name: "structured string", want: "Accepted: Yes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(test.want, auditText(test.input))
		})
	}
}
