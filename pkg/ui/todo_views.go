package ui

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/db"
)

//go:embed static/todo.css
var todoCSS string

type TodoProps struct {
	ActiveCount    int
	CompletedCount int
	EditingID      int64
	Filter         string
	Tasks          []db.Task
}

func TodoCSS() templ.Component {
	return templ.Raw(`<style>` + todoCSS + `</style>`)
}

func itoa(n int) string { return strconv.Itoa(n) }

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func taskID(t db.Task) string { return strconv.FormatInt(t.ID, 10) }

func taskPath(t db.Task) string { return "/tasks/" + taskID(t) }

func toggleVals(t db.Task) string { return fmt.Sprintf(`{"completed":%t}`, !t.Completed) }
