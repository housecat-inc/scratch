package todo

import (
	"github.com/housecat-inc/scratch/pkg/db"
)

type Filter string

const (
	FilterActive    Filter = "active"
	FilterAll       Filter = "all"
	FilterCompleted Filter = "completed"
)

func ParseFilter(s string) Filter {
	switch Filter(s) {
	case FilterActive:
		return FilterActive
	case FilterCompleted:
		return FilterCompleted
	default:
		return FilterAll
	}
}

type View struct {
	ActiveCount    int
	CompletedCount int
	Filter         Filter
	Tasks          []db.Task
}

type Service struct {
	tasks db.TaskStore
}

func NewService(tasks db.TaskStore) *Service {
	return &Service{tasks: tasks}
}

func (s *Service) All() ([]db.Task, error) {
	return s.tasks.ListTasks()
}

func (s *Service) ClearCompleted() (int, error) {
	return s.tasks.ClearCompletedTasks()
}

func (s *Service) Create(title string) (db.Task, error) {
	return s.tasks.AddTask(title)
}

func (s *Service) Delete(id int64) error {
	return s.tasks.DeleteTask(id)
}

func (s *Service) Edit(id int64, title string) (db.Task, error) {
	return s.tasks.UpdateTaskTitle(id, title)
}

func (s *Service) Get(id int64) (db.Task, error) {
	return s.tasks.GetTask(id)
}

func (s *Service) SetCompleted(id int64, completed bool) (db.Task, error) {
	return s.tasks.SetTaskCompleted(id, completed)
}

func (s *Service) Toggle(id int64) (db.Task, error) {
	task, err := s.tasks.GetTask(id)
	if err != nil {
		return db.Task{}, err
	}
	return s.tasks.SetTaskCompleted(id, !task.Completed)
}

func (s *Service) View(filter Filter) (View, error) {
	all, err := s.tasks.ListTasks()
	if err != nil {
		return View{}, err
	}

	view := View{Filter: filter, Tasks: make([]db.Task, 0, len(all))}
	for _, task := range all {
		if task.Completed {
			view.CompletedCount++
		} else {
			view.ActiveCount++
		}
		switch filter {
		case FilterActive:
			if task.Completed {
				continue
			}
		case FilterCompleted:
			if !task.Completed {
				continue
			}
		}
		view.Tasks = append(view.Tasks, task)
	}
	return view, nil
}
