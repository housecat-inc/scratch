package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const reviewTimeout = 24 * time.Hour

type Contact struct {
	Company string `json:"company,omitempty" jsonschema:"Company or organization"`
	Email   string `json:"email" jsonschema:"Email address"`
	Name    string `json:"name" jsonschema:"Full name"`
	Notes   string `json:"notes,omitempty" jsonschema:"Notes for the follow-up"`
}

type ContactInput struct {
	MessageID int64
	Prompt    string
	ThreadID  int64
}

type Deps struct {
	DBOS    dbos.DBOSContext
	Log     *slog.Logger
	Publish func(threadID int64)
	Store   db.ThreadStore
	Tasks   db.TaskStore
}

type Flows struct {
	deps Deps
}

func New(deps Deps) *Flows {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	f := &Flows{deps: deps}
	dbos.RegisterWorkflow(deps.DBOS, f.contactIntake, dbos.WithWorkflowName("contact-intake"))
	return f
}

func (f *Flows) Agent() chat.Agent { return flowAgent{flows: f} }

func (f *Flows) Deliver(workflowID, topic, idempotencyKey string, reply elicit.Reply) error {
	err := dbos.Send(f.deps.DBOS, workflowID, reply, topic, dbos.WithIdempotencyKey(idempotencyKey))
	return errors.Wrap(err, "send reply")
}

func (f *Flows) contactIntake(ctx dbos.DBOSContext, in ContactInput) (string, error) {
	if err := f.toolCall(ctx, in, "draft_contact", map[string]string{"prompt": in.Prompt}); err != nil {
		return "", err
	}
	draft, err := dbos.RunAsStep(ctx, func(context.Context) (Contact, error) {
		return draftContact(in.Prompt), nil
	}, dbos.WithStepName("draft_contact"))
	if err != nil {
		return "", errors.Wrap(err, "draft contact")
	}
	if err := f.delta(ctx, in, "Drafted a contact from your message — review the details below.\n"); err != nil {
		return "", err
	}

	contact, action, err := elicit.Form(ctx, prompter{deps: f.deps, in: in}, "review",
		"Review the contact before I save it.", draft, reviewTimeout)
	if err != nil {
		return "", errors.Wrap(err, "elicit review")
	}

	switch action {
	case elicit.ActionAccept:
		if err := f.toolCall(ctx, in, "add_task", map[string]string{"email": contact.Email, "name": contact.Name}); err != nil {
			return "", err
		}
		task, err := dbos.RunAsStep(ctx, func(context.Context) (db.Task, error) {
			return f.deps.Tasks.AddTask(followUpTitle(contact))
		}, dbos.WithStepName("add_task"))
		if err != nil {
			return "", errors.Wrap(err, "add task")
		}
		if err := f.delta(ctx, in, fmt.Sprintf("Saved. Added todo #%d: %s", task.ID, task.Title)); err != nil {
			return "", err
		}
	case elicit.ActionDecline:
		if err := f.delta(ctx, in, "Okay — discarded the contact draft."); err != nil {
			return "", err
		}
	default:
		if err := f.delta(ctx, in, "The review form was dismissed without a decision."); err != nil {
			return "", err
		}
	}

	if err := f.finish(ctx, in); err != nil {
		return "", err
	}
	return action, nil
}

func (f *Flows) delta(ctx dbos.DBOSContext, in ContactInput, text string) error {
	_, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		ev := chat.DeltaEvent(text)
		if _, err := f.deps.Store.AddMessageEvent(in.MessageID, ev.Type, ev.Data); err != nil {
			return "", err
		}
		if err := f.deps.Store.AppendMessageBody(in.MessageID, text); err != nil {
			return "", err
		}
		f.deps.Publish(in.ThreadID)
		return "", nil
	}, dbos.WithStepName("delta"))
	return errors.Wrap(err, "emit delta")
}

func (f *Flows) finish(ctx dbos.DBOSContext, in ContactInput) error {
	_, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		if _, err := f.deps.Store.FinishMessage(in.MessageID, db.MessageStatusComplete); err != nil {
			return "", err
		}
		if err := f.deps.Store.TouchThread(in.ThreadID); err != nil {
			return "", err
		}
		f.deps.Publish(in.ThreadID)
		return "", nil
	}, dbos.WithStepName("finish"))
	return errors.Wrap(err, "finish message")
}

func (f *Flows) toolCall(ctx dbos.DBOSContext, in ContactInput, name string, input map[string]string) error {
	_, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		data, err := json.Marshal(map[string]any{"input": input, "name": name})
		if err != nil {
			return "", err
		}
		if _, err := f.deps.Store.AddMessageEvent(in.MessageID, chat.EventToolCall, string(data)); err != nil {
			return "", err
		}
		f.deps.Publish(in.ThreadID)
		return "", nil
	}, dbos.WithStepName("tool_call/"+name))
	return errors.Wrap(err, "emit tool call")
}

type flowAgent struct {
	flows *Flows
}

func (flowAgent) Author() string { return "workflow:contact" }

func (a flowAgent) Run(ctx context.Context, turn chat.Turn, emit func(chat.Event)) (string, error) {
	handle, err := dbos.RunWorkflow(a.flows.deps.DBOS, a.flows.contactIntake, ContactInput{
		MessageID: turn.MessageID,
		Prompt:    turn.Prompt,
		ThreadID:  turn.ThreadID,
	}, dbos.WithWorkflowID(fmt.Sprintf("chat-message-%d", turn.MessageID)))
	if err != nil {
		return "", errors.Wrap(err, "run contact intake")
	}
	done := make(chan error, 1)
	go func() {
		_, err := handle.GetResult()
		done <- err
	}()
	select {
	case <-ctx.Done():
		return "", chat.ErrTurnPending
	case err := <-done:
		return "", errors.Wrap(err, "contact intake")
	}
}

type prompter struct {
	deps Deps
	in   ContactInput
}

func (p prompter) Prompt(pr elicit.Prompt) error {
	data, err := json.Marshal(pr)
	if err != nil {
		return errors.Wrap(err, "marshal prompt")
	}
	if _, err := p.deps.Store.AddMessageEvent(p.in.MessageID, chat.EventElicitation, string(data)); err != nil {
		return errors.Wrap(err, "record elicitation")
	}
	p.deps.Publish(p.in.ThreadID)
	return nil
}

var emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

var nameStopwords = map[string]bool{
	"a": true, "add": true, "contact": true, "create": true, "for": true,
	"from": true, "intake": true, "meet": true, "met": true, "new": true,
	"please": true, "save": true, "the": true, "with": true,
}

func draftContact(prompt string) Contact {
	contact := Contact{Email: emailPattern.FindString(prompt)}
	words := strings.Fields(strings.ReplaceAll(prompt, contact.Email, ""))
	var name []string
	for _, word := range words {
		word = strings.Trim(word, ",.;:()<>\"'")
		if nameStopwords[strings.ToLower(word)] {
			if len(name) > 0 {
				break
			}
			continue
		}
		runes := []rune(word)
		if len(runes) > 1 && unicode.IsUpper(runes[0]) {
			name = append(name, word)
			continue
		}
		if len(name) > 0 {
			break
		}
	}
	contact.Name = strings.Join(name, " ")
	return contact
}

func followUpTitle(c Contact) string {
	if c.Name == "" {
		return "Follow up with " + c.Email
	}
	return fmt.Sprintf("Follow up with %s <%s>", c.Name, c.Email)
}
