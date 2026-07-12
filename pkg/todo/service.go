package todo

import (
	"github.com/housecat-inc/scratch/pkg/db"
)

type Filter string

const (
	FilterActive    Filter = "active"
	FilterAll       Filter = "all"
	FilterArchived  Filter = "archived"
	FilterCompleted Filter = "completed"
	FilterDone      Filter = "done"
)

func ParseFilter(s string) Filter {
	switch Filter(s) {
	case FilterActive:
		return FilterActive
	case FilterArchived:
		return FilterArchived
	case FilterCompleted:
		return FilterCompleted
	case FilterDone:
		return FilterCompleted
	default:
		return FilterAll
	}
}

type Detail struct {
	Notes    []db.TaskNote
	Subitems []db.TaskSubitem
	Task     db.Task
}

type View struct {
	ActiveCount    int
	ArchivedCount  int
	CompletedCount int
	Detail         *Detail
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

func (s *Service) AddNote(taskID int64, body string) (db.TaskNote, error) {
	if _, err := s.tasks.GetTask(taskID); err != nil {
		return db.TaskNote{}, err
	}
	return s.tasks.AddTaskNote(taskID, body)
}

func (s *Service) AddSubitem(taskID int64, title string) (db.TaskSubitem, error) {
	if _, err := s.tasks.GetTask(taskID); err != nil {
		return db.TaskSubitem{}, err
	}
	return s.tasks.AddTaskSubitem(taskID, title)
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

func (s *Service) DeleteSubitem(id int64) error {
	return s.tasks.DeleteTaskSubitem(id)
}

func (s *Service) Edit(id int64, title string) (db.Task, error) {
	return s.tasks.UpdateTaskTitle(id, title)
}

func (s *Service) EditDescription(id int64, description string) (db.Task, error) {
	return s.tasks.UpdateTaskDescription(id, description)
}

func (s *Service) Get(id int64) (db.Task, error) {
	return s.tasks.GetTask(id)
}

func (s *Service) SetArchived(id int64, archived bool) (db.Task, error) {
	return s.tasks.SetTaskArchived(id, archived)
}

func (s *Service) SetCompleted(id int64, completed bool) (db.Task, error) {
	return s.tasks.SetTaskCompleted(id, completed)
}

func (s *Service) SetStarred(id int64, starred bool) (db.Task, error) {
	return s.tasks.SetTaskStarred(id, starred)
}

func (s *Service) SetSubitemCompleted(id int64, completed bool) (db.TaskSubitem, error) {
	return s.tasks.SetTaskSubitemCompleted(id, completed)
}

func (s *Service) Toggle(id int64) (db.Task, error) {
	task, err := s.tasks.GetTask(id)
	if err != nil {
		return db.Task{}, err
	}
	return s.tasks.SetTaskCompleted(id, !task.Completed)
}

func (s *Service) View(filter Filter) (View, error) {
	return s.ViewTask(filter, 0)
}

func (s *Service) ViewTask(filter Filter, selectedID int64) (View, error) {
	all, err := s.tasks.ListTasks()
	if err != nil {
		return View{}, err
	}

	view := View{Filter: filter, Tasks: make([]db.Task, 0, len(all))}
	for _, task := range all {
		if task.Archived {
			view.ArchivedCount++
		}
		if task.Completed {
			view.CompletedCount++
		}
		if !task.Archived && !task.Completed {
			view.ActiveCount++
		}
		switch filter {
		case FilterActive:
			if task.Archived || task.Completed {
				continue
			}
		case FilterArchived:
			if !task.Archived {
				continue
			}
		case FilterCompleted:
			if task.Archived || !task.Completed {
				continue
			}
		default:
			if task.Archived {
				continue
			}
		}
		view.Tasks = append(view.Tasks, task)
	}
	if selectedID == 0 && len(view.Tasks) > 0 {
		selectedID = view.Tasks[0].ID
	}
	if selectedID != 0 {
		detail, err := s.detail(selectedID)
		if err != nil {
			return View{}, err
		}
		view.Detail = &detail
	}
	return view, nil
}

func (s *Service) detail(taskID int64) (Detail, error) {
	task, err := s.tasks.GetTask(taskID)
	if err != nil {
		return Detail{}, err
	}
	notes, err := s.tasks.ListTaskNotes(taskID)
	if err != nil {
		return Detail{}, err
	}
	subitems, err := s.tasks.ListTaskSubitems(taskID)
	if err != nil {
		return Detail{}, err
	}
	return Detail{
		Notes:    notes,
		Subitems: subitems,
		Task:     task,
	}, nil
}
