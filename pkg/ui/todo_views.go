package ui

import "github.com/housecat-inc/scratch/pkg/db"

type TodoProps struct {
	ActiveCount int
	Detail      *TodoTaskDetail
	Title       string
	Tasks       []db.Task
	TaskCount   int
	View        string
}

type TodoTaskDetail struct {
	Task db.Task
}
