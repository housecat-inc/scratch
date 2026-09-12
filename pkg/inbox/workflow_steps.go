package inbox

import "github.com/housecat-inc/scratch/pkg/workflow"

func workflowSteps(name string) []workflow.StepDefinition {
	if name == "workflow:contact" {
		return []workflow.StepDefinition{
			{Name: "tool_call/draft_contact", Description: "Record the contact-drafting tool call in the conversation."},
			{Name: "draft_contact", Description: "Extract a contact from your message, using a basic text parser if extraction fails."},
			{Name: "delta", Description: "Post a message explaining that the contact draft is ready for review."},
			{Name: "elicit/review", Description: "Show the contact review form."},
			{Name: "DBOS.recv", Description: "Wait up to 24 hours for your review decision."},
			{Name: "tool_call/add_task", Description: "If accepted: record the task-creation tool call."},
			{Name: "add_task", Description: "If accepted: create a follow-up task using the reviewed contact."},
			{Name: "delta", Description: "Post the outcome: task created, draft declined, or review dismissed or timed out."},
			{Name: "finish", Description: "Mark the response complete and update the conversation."},
		}
	}
	return nil
}
