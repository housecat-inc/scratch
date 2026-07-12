package todo

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

var screenshots = []testkit.Screenshot{
	{Height: 480, Name: "todos.png", Width: 640},
}

func TestTodoMVC(t *testing.T) {
	run(t, []Case{
		{
			Name:        "adds tasks and counts them",
			Path:        "/",
			Screenshots: screenshots,
			Act: []Step{
				Type("#new-todo", "buy milk"),
				Press("#new-todo", "Enter"),
				Type("#new-todo", "walk dog"),
				Press("#new-todo", "Enter"),
			},
			Assert: []Step{
				TextContains("#todo-list", "buy milk"),
				TextContains("#todo-list", "walk dog"),
				TextContains("#todo-count", "2 items left"),
				Log(`{"level":"INFO","method":"POST","msg":"http.request","path":"/tasks","status":200}`),
				Metric(`{"msg":"http.request","method":"POST","path":"/tasks","status":200}`, 2),
			},
		},
		{
			Name: "toggles then clears completed",
			Path: "/",
			Seed: []Step{SeedTasks("write tests")},
			Act: []Step{
				Click(".todo-item .toggle"),
			},
			Assert: []Step{
				ClassContains(".todo-item", "completed"),
				Visible("#clear-completed"),
				TextContains("#todo-count", "0 items left"),
			},
		},
		{
			Name: "clears completed leaves empty state",
			Path: "/",
			Seed: []Step{SeedTasks("throwaway")},
			Act: []Step{
				Click(".todo-item .toggle"),
				Click("#clear-completed"),
			},
			Assert: []Step{
				TextContains("#todo-main", "Nothing to do"),
				ElementAbsent(".todo-item"),
			},
		},
		{
			Name: "edits a task title inline",
			Path: "/",
			Seed: []Step{SeedTasks("draft")},
			Act: []Step{
				Click(".todo-item .edit"),
				Visible(".edit-input"),
				Type(".edit-input", " done"),
				Press(".edit-input", "Enter"),
			},
			Assert: []Step{
				TextContains("#todo-list", "done"),
				ElementAbsent(".edit-input"),
			},
		},
		{
			Name: "deletes a task",
			Path: "/",
			Seed: []Step{SeedTasks("temporary")},
			Act: []Step{
				Click(".todo-item .destroy"),
			},
			Assert: []Step{
				TextContains("#todo-main", "Nothing to do"),
			},
		},
		{
			Name: "filters completed and active",
			Path: "/",
			Seed: []Step{SeedTasks("active one", "done one")},
			Act: []Step{
				Click("#todo-list li:last-child .toggle"),
				ClassContains("#todo-list li:last-child", "completed"),
				Click("#filter-completed"),
			},
			Assert: []Step{
				TextContains("#todo-list", "done one"),
				ElementAbsent("#todo-list li:nth-child(2)"),
			},
		},
	})
}
