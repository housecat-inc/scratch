package workflow

import (
	"context"
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

func (w *Workflows) ConfigureExamples() error {
	return w.RegisterSchedule(ScheduleDefinition{
		Description: "A local example that records a greeting in run history. Run it now to explore results and step audits, or resume its daily schedule. No accounts or external services are required.",
		Name:        "example-greeting",
		Run:         scheduledGreeting,
		Schedule:    "0 0 9 * * *",
		Schema:      []SummaryField{{Label: "Greeting", Path: "output.greeting"}, {Label: "Created", Path: "output.created"}},
		Steps:       []StepDefinition{{Description: "Compose and record a greeting.", Name: "compose-greeting"}},
		Timezone:    "UTC",
		Title:       "Example greeting",
	})
}

type greetingResult struct {
	Created  string `json:"created"`
	Greeting string `json:"greeting"`
}

func scheduledGreeting(ctx dbos.DBOSContext, input dbos.ScheduledWorkflowInput) (any, error) {
	return dbos.RunAsStep(ctx, func(context.Context) (greetingResult, error) {
		return greetingResult{Created: time.Now().UTC().Format(time.RFC3339), Greeting: "Hello from Scratch! Your workflow ran successfully."}, nil
	}, dbos.WithStepName("compose-greeting"))
}
