package ui

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
	Counts NavCounts
	Runs   []WorkflowRunItem
}

type WorkflowRunProps struct {
	Counts NavCounts
	Detail InboxWorkflowDetail
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
