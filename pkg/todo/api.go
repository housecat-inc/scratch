package todo

import (
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"github.com/housecat-inc/scratch/pkg/db"
)

type Task struct {
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"createdAt"`
	Description string    `json:"description"`
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TaskCreateIn struct {
	Description string `json:"description"`
	Title       string `json:"title"`
}

type TaskUpdateIn struct {
	Completed   *bool   `json:"completed"`
	Description *string `json:"description"`
	Title       *string `json:"title"`
}

type handlers struct {
	svc *Service
}

func Register(s *fuego.Server, svc *Service) {
	h := handlers{svc: svc}
	fuego.Get(s, "/api/tasks", h.listTasks, option.Summary("List tasks"), option.Tags("tasks"))
	fuego.Post(s, "/api/tasks", h.createTask, option.Summary("Create a task"), option.Tags("tasks"))
	fuego.Get(s, "/api/tasks/{id}", h.getTask, option.Summary("Get a task"), option.Tags("tasks"))
	fuego.Patch(s, "/api/tasks/{id}", h.updateTask, option.Summary("Update a task"), option.Tags("tasks"))
	fuego.Delete(s, "/api/tasks/{id}", h.deleteTask, option.Summary("Delete a task"), option.Tags("tasks"))

	dropUnreferencedSchemas(s)
}

func dropUnreferencedSchemas(s *fuego.Server) {
	delete(s.OpenAPI.Description().Components.Schemas, "unknown-interface")
}

func (h handlers) createTask(c fuego.ContextWithBody[TaskCreateIn]) (Task, error) {
	body, err := c.Body()
	if err != nil {
		return Task{}, errors.Wrap(err, "read body")
	}
	task, err := h.svc.Create(body.Title)
	if err != nil {
		return Task{}, errors.Wrap(err, "create task")
	}
	if body.Description != "" {
		task, err = h.svc.EditDescription(task.ID, body.Description)
		if err != nil {
			return Task{}, errors.Wrap(err, "set description")
		}
	}
	return toResponse(task), nil
}

func (h handlers) deleteTask(c fuego.ContextNoBody) (Task, error) {
	id, err := pathID(c)
	if err != nil {
		return Task{}, err
	}
	if err := h.svc.Delete(id); err != nil {
		return Task{}, notFoundOr(err, "delete task")
	}
	return Task{}, nil
}

func (h handlers) getTask(c fuego.ContextNoBody) (Task, error) {
	id, err := pathID(c)
	if err != nil {
		return Task{}, err
	}
	task, err := h.svc.Get(id)
	if err != nil {
		return Task{}, notFoundOr(err, "get task")
	}
	return toResponse(task), nil
}

func (h handlers) listTasks(c fuego.ContextNoBody) ([]Task, error) {
	tasks, err := h.svc.All()
	if err != nil {
		return nil, errors.Wrap(err, "list tasks")
	}
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, toResponse(task))
	}
	return out, nil
}

func (h handlers) updateTask(c fuego.ContextWithBody[TaskUpdateIn]) (Task, error) {
	body, err := c.Body()
	if err != nil {
		return Task{}, errors.Wrap(err, "read body")
	}
	id, err := pathID(c)
	if err != nil {
		return Task{}, err
	}
	if body.Title != nil {
		if _, err := h.svc.Edit(id, *body.Title); err != nil {
			return Task{}, notFoundOr(err, "update title")
		}
	}
	if body.Description != nil {
		if _, err := h.svc.EditDescription(id, *body.Description); err != nil {
			return Task{}, notFoundOr(err, "update description")
		}
	}
	if body.Completed != nil {
		if _, err := h.svc.SetCompleted(id, *body.Completed); err != nil {
			return Task{}, notFoundOr(err, "set completed")
		}
	}
	task, err := h.svc.Get(id)
	if err != nil {
		return Task{}, notFoundOr(err, "get task")
	}
	return toResponse(task), nil
}

func notFoundOr(err error, msg string) error {
	if db.IsTaskNotFound(err) {
		return fuego.NotFoundError{Detail: "task not found", Err: errors.Wrap(err, msg)}
	}
	return errors.Wrap(err, msg)
}

func pathID(c fuego.ContextWithPathParam) (int64, error) {
	id, err := strconv.ParseInt(c.PathParam("id"), 10, 64)
	if err != nil {
		return 0, fuego.BadRequestError{Detail: "id must be an integer", Err: errors.Wrap(err, "parse id")}
	}
	return id, nil
}

func toResponse(t db.Task) Task {
	return Task{
		Completed:   t.Completed,
		CreatedAt:   t.CreatedAt.Time,
		Description: t.Description,
		ID:          t.ID,
		Title:       t.Title,
		UpdatedAt:   t.UpdatedAt.Time,
	}
}
