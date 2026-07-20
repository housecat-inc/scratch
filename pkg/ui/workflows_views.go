package ui

import "github.com/housecat-inc/scratch/uikit"

type WorkflowStartOption struct {
	Label string
	Type  string
}

type WorkflowRunItem struct {
	Href   string
	ID     int64
	Status string
	Title  string
	When   string
}

type WorkflowsProps struct {
	ChatLabel   string
	ChatOptions []uikit.SelectOption
	Counts      NavCounts
	Runs        []WorkflowRunItem
}

type WorkflowRunProps struct {
	ChatLabel   string
	ChatOptions []uikit.SelectOption
	Counts      NavCounts
	Detail      InboxWorkflowDetail
}

func TaskWorkflowOptions() []WorkflowStartOption {
	types := []string{"contact-note", "greet", "countdown"}
	out := make([]WorkflowStartOption, 0, len(types))
	for _, t := range types {
		out = append(out, WorkflowStartOption{Label: workflowTypeLabel(t), Type: t})
	}
	return out
}

func WorkflowLabel(typ string) string {
	return workflowTypeLabel(typ)
}
