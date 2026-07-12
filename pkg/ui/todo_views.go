package ui

import "github.com/housecat-inc/scratch/pkg/db"

type TodoProps struct {
	ActiveCount int
	Detail      *TodoTaskDetail
	Tasks       []db.Task
}

type TodoTaskDetail struct {
	Task db.Task
}
