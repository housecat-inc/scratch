# How to build workflows

Scratch workflows are Go functions executed by DBOS. They accept serializable inputs, checkpoint work in named steps, and return a result or error. DBOS stores durable execution records alongside application data in SQLite.

## Included examples

| Workflow | Trigger | Result |
| --- | --- | --- |
| Greet | API | A greeting from `/api/workflows/greet` |
| Example greeting | Run now or daily at 09:00 UTC; initially paused | A greeting and timestamp in run history |
| Contact intake | Chat with the Contact workflow | Draft a contact, wait for review, and create one follow-up task if accepted |

The New workflow wizard gathers a description, timing, and timezone and starts an agent build conversation. A coding agent still needs to implement and register the workflow.

## Register a scheduled workflow

Implement the signature `func(dbos.DBOSContext, dbos.ScheduledWorkflowInput) (any, error)` and register it before `Launch`:

```go
err := workflows.RegisterSchedule(workflow.ScheduleDefinition{
    Description: "Import records and report validation errors.",
    Name: "import-records",
    Run: importRecords,
    Schedule: "0 0 9 * * *",
    Schema: []workflow.SummaryField{
        {Label: "Saved", Path: "output.saved"},
        {Label: "Errors", Path: "output.errors"},
    },
    Steps: []workflow.StepDefinition{
        {Description: "Capture the source records.", Name: "fetch-records"},
        {Description: "Validate the captured data.", Name: "validate-records"},
        {Description: "Persist valid records with deduplication keys.", Name: "persist-records"},
    },
    Timezone: "UTC",
    Title: "Record imports",
})
```

Handle registration errors. After launch, `EnsureSchedules` creates missing schedules, disables backfill, and pauses newly created schedules. Existing schedule expressions, timezones, and pause states are preserved on startup. Resume explicitly in the UI when ready. The DBOS version used here creates active schedules before they can be paused, so schedule creation and pausing are separate operations; do not depend on initial pause as authorization for an external write.

`inbox.ConfigureWorkflows` exposes registered definitions in the Workflows UI and reuses the matching workflow thread on restart. Use stable, unique workflow names. The UI shows the actual persisted schedule, timezone, next run, status, recent runs, totals, success rate, and Run now/Pause/Resume controls. Edit opens a chat about changing the schedule or trigger.

See [the registry](../pkg/workflow/schedules.go) and [the example](../pkg/workflow/examples.go). External triggers and custom inputs require an explicit handler that invokes `dbos.RunWorkflow`; they are not automatically provisioned by schedule registration.

## Durable steps

Keep orchestration deterministic. Put network calls, database operations, model calls, clock reads, and other side effects inside `dbos.RunAsStep`. Persist serializable outputs so recovery can reuse completed work. Split independent operations at meaningful recovery boundaries; use per-item steps or child workflows for independently recoverable items.

Use operation timeouts through `context.WithTimeout` inside steps and bounded retries with backoff for transient failures. Stop on permanent configuration or validation errors. The greeting is a single local operation and needs only one step; a multi-operation import needs separate fetch, validation, and persistence steps.

Durability alone does not guarantee exactly-once external writes. Use stable idempotency keys, uniqueness constraints, or reconciliation for the window between an external effect and its DBOS checkpoint. Contact intake uses transactional `workflow_effects` receipts for tasks and chat events.

## Results and audits

A schedule may declare two or three labeled summary fields using `output`, `output.field`, or `input.field` paths. Without a schema, the table shows Output. Arrays show their item count; zero and false remain visible. Status and timestamps are always present.

Open a run timestamp to see its recorded step names, statuses, inputs, outputs, and errors. `/workflows/audit` also lists executions with pagination. The step overview describes the implementation; each run's audit shows what actually executed. Keep step descriptions aligned with executable DBOS steps. Refresh to see newer run state.

## Agent work and human review

Run model or agent calls inside bounded steps and validate their outputs before use. [Contact extraction](../pkg/flow/extract.go) uses a structured extraction call with a local parser fallback. There is no shared primitive here for resuming a Scratch ACP session from a workflow.

Use [`elicit.Form`](../pkg/elicit/form.go) to show a typed review form and wait through DBOS messaging. [Contact intake](../pkg/flow/flow.go) handles acceptance, decline, and timeout, and creates a task only after acceptance.

## Verify recovery

Use temporary databases and isolated fixtures for writes. Interrupt a run after a completed step, restart against the same database, and verify that completed work is reused and effects are not duplicated. Also check failed runs, run history, and schedule state after restart.

The [contact restart test](../pkg/flow/restart_test.go) verifies a pending review survives restart and creates one task. The [effect tests](../pkg/db/workflow_effects_test.go) check atomic receipt writes and duplicate prevention. The [schedule tests](../pkg/workflow/schedules_test.go) verify history, timezone, schedule, and pause state survive restart. The [inbox tests](../pkg/inbox/scheduled_workflow_test.go) exercise schedule controls and results.
