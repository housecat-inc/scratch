package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const (
	titleMaxLen = 80
	userAuthor  = "you"
)

var (
	ErrAgentUnknown        = errors.New("agent unknown")
	ErrElicitationNotFound = errors.New("elicitation not found")
	ErrElicitationResolved = errors.New("elicitation already resolved")
	ErrNoResolver          = errors.New("no elicitation resolver configured")
	ErrThreadBusy          = errors.New("thread busy")
	ErrThreadStateUnknown  = errors.New("thread state unknown")
	ErrTurnPending         = errors.New("turn pending durable recovery")
)

func IsThreadBusy(err error) bool { return errors.Is(err, ErrThreadBusy) }

type Recoverable interface {
	Alive(turn Turn) bool
}

type Resolver interface {
	Deliver(workflowID, topic, idempotencyKey string, reply elicit.Reply) error
}

type ThreadView struct {
	Attachments map[int64][]db.Attachment
	Forms       map[int64]elicit.Prompt
	Messages    []db.Message
	Parts       map[int64][]MessagePart
	Streaming   bool
	Thread      db.Thread
}

type Service struct {
	agents      map[string]Agent
	broker      *Broker
	cancel      context.CancelFunc
	ctx         context.Context
	defaultName string
	log         *slog.Logger
	resolver    Resolver
	store       db.ThreadStore
	wg          sync.WaitGroup
}

func NewService(store db.ThreadStore, agent Agent, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	name := AgentName(agent)
	return &Service{
		agents:      map[string]Agent{name: agent},
		broker:      NewBroker(),
		cancel:      cancel,
		ctx:         ctx,
		defaultName: name,
		log:         log,
		store:       store,
	}
}

func (s *Service) AgentName(thread db.Thread) string {
	var anchor struct {
		Agent string `json:"agent"`
	}
	_ = json.Unmarshal([]byte(thread.Anchor), &anchor)
	if anchor.Agent == "" {
		return s.defaultName
	}
	return anchor.Agent
}

func (s *Service) AgentNames() []string {
	names := make([]string, 0, len(s.agents))
	names = append(names, s.defaultName)
	for name := range s.agents {
		if name != s.defaultName {
			names = append(names, name)
		}
	}
	slices.Sort(names[1:])
	return names
}

func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Service) CreateThread(agent, title string) (db.Thread, error) {
	return s.CreateThreadWithModel(agent, "", title)
}

func (s *Service) CreateThreadWithModel(agent, model, title string) (db.Thread, error) {
	if agent == "" {
		agent = s.defaultName
	}
	if _, ok := s.agents[agent]; !ok {
		return db.Thread{}, errors.Wrapf(ErrAgentUnknown, "%q", agent)
	}
	anchorData := map[string]string{"agent": agent}
	if strings.TrimSpace(model) != "" {
		anchorData["model"] = strings.TrimSpace(model)
	}
	anchor, err := json.Marshal(anchorData)
	if err != nil {
		return db.Thread{}, errors.Wrap(err, "marshal anchor")
	}
	return s.store.AddThread(db.ThreadKindChat, strings.TrimSpace(title), string(anchor))
}

func (s *Service) DeleteThread(threadID int64) error {
	return s.store.DeleteThread(threadID)
}

func (s *Service) Publish(threadID int64) {
	s.broker.Publish(threadID, StructuralUpdate)
}

func (s *Service) Recover() error {
	msgs, err := s.store.ListMessagesByStatus(db.MessageStatusStreaming)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		thread, err := s.store.GetThread(m.ThreadID)
		if err != nil {
			return err
		}
		if thread.Kind != db.ThreadKindChat {
			continue
		}
		if agent, err := s.resolveAgent(thread); err == nil {
			turn := Turn{Anchor: thread.Anchor, MessageID: m.ID, Meta: m.Meta, ThreadID: m.ThreadID}
			if recoverable, ok := agent.(Recoverable); ok && recoverable.Alive(turn) {
				continue
			}
			if HasRunner(m.Meta) {
				if err := s.reattach(agent, thread, turn); err != nil {
					return err
				}
				continue
			}
		}
		s.emit(m.ThreadID, m.ID, DeltaEvent("\nThe turn was interrupted by a restart."))
		if _, err := s.store.FinishMessage(m.ID, db.MessageStatusError); err != nil {
			return err
		}
		s.log.Info("chat.recovered", "message", m.ID, "thread", m.ThreadID)
	}
	return nil
}

func (s *Service) reattach(agent Agent, thread db.Thread, turn Turn) error {
	if err := s.store.ResetMessageEvents(turn.MessageID); err != nil {
		return err
	}
	if err := s.store.SetMessageBody(turn.MessageID, ""); err != nil {
		return err
	}
	s.wg.Add(1)
	go s.run(agent, thread, turn)
	s.log.Info("chat.reattached", "message", turn.MessageID, "thread", thread.ID)
	return nil
}

func (s *Service) RegisterAgent(name string, agent Agent) {
	s.agents[name] = agent
}

func (s *Service) ResolveElicitation(messageID int64, elicitationID, action string, values map[string]string) error {
	if s.resolver == nil {
		return ErrNoResolver
	}
	msg, err := s.store.GetMessage(messageID)
	if err != nil {
		return err
	}
	events, err := s.store.ListMessageEvents(messageID, 0)
	if err != nil {
		return err
	}
	var prompt *elicit.Prompt
	for _, ev := range events {
		switch ev.Type {
		case EventElicitation:
			var p elicit.Prompt
			if err := json.Unmarshal([]byte(ev.Data), &p); err == nil && p.ElicitationID == elicitationID {
				prompt = &p
			}
		case EventElicitationResult:
			var r elicitationResult
			if err := json.Unmarshal([]byte(ev.Data), &r); err == nil && r.ElicitationID == elicitationID {
				return ErrElicitationResolved
			}
		}
	}
	if prompt == nil {
		return ErrElicitationNotFound
	}

	reply := elicit.Reply{Action: action}
	switch action {
	case elicit.ActionAccept:
		content, err := elicit.Coerce(prompt.RequestedSchema, values)
		if err != nil {
			return err
		}
		data, err := json.Marshal(content)
		if err != nil {
			return errors.Wrap(err, "marshal content")
		}
		reply.Content = string(data)
	case elicit.ActionCancel, elicit.ActionDecline:
	default:
		return errors.Wrapf(elicit.ErrInvalid, "unknown action %q", action)
	}

	result, err := json.Marshal(elicitationResult{Action: action, Content: reply.Content, ElicitationID: elicitationID})
	if err != nil {
		return errors.Wrap(err, "marshal result")
	}
	if _, err := s.store.AddMessageEvent(messageID, EventElicitationResult, string(result)); err != nil {
		return errors.Wrap(err, "record result")
	}
	if err := s.resolver.Deliver(prompt.WorkflowID, prompt.Topic, prompt.ElicitationID, reply); err != nil {
		return errors.Wrap(err, "deliver reply")
	}
	s.broker.Publish(msg.ThreadID, messageID)
	return nil
}

func (s *Service) Send(threadID int64, prompt string, attachmentIDs ...int64) (db.Message, error) {
	thread, err := s.store.GetThread(threadID)
	if err != nil {
		return db.Message{}, err
	}
	agent, err := s.resolveAgent(thread)
	if err != nil {
		return db.Message{}, err
	}
	msgs, err := s.store.ListThreadMessages(threadID)
	if err != nil {
		return db.Message{}, err
	}
	for _, m := range msgs {
		if m.Status == db.MessageStatusStreaming {
			return db.Message{}, ErrThreadBusy
		}
	}

	user, err := s.store.AddMessage(db.NewMessage{
		Author:   userAuthor,
		Body:     prompt,
		Role:     db.MessageRoleUser,
		ThreadID: threadID,
	})
	if err != nil {
		return db.Message{}, err
	}
	if err := s.store.LinkAttachments(threadID, user.ID, attachmentIDs); err != nil {
		return db.Message{}, err
	}
	files, err := s.attachmentPaths(user.ID, attachmentIDs)
	if err != nil {
		return db.Message{}, err
	}
	asst, err := s.store.AddMessage(db.NewMessage{
		Author:   agent.Author(),
		ParentID: &user.ID,
		Role:     db.MessageRoleAssistant,
		Status:   db.MessageStatusStreaming,
		ThreadID: threadID,
	})
	if err != nil {
		return db.Message{}, err
	}
	if thread.Title == "" {
		if err := s.store.SetThreadTitle(threadID, truncate(prompt, titleMaxLen)); err != nil {
			return db.Message{}, err
		}
	} else if err := s.store.TouchThread(threadID); err != nil {
		return db.Message{}, err
	}
	s.broker.Publish(threadID, StructuralUpdate)

	turn := Turn{
		Anchor:      thread.Anchor,
		Attachments: files,
		MessageID:   asst.ID,
		Meta:        asst.Meta,
		Prompt:      prompt,
		ThreadID:    threadID,
	}
	s.wg.Add(1)
	go s.run(agent, thread, turn)
	return user, nil
}

func (s *Service) attachmentPaths(messageID int64, ids []int64) ([]Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.store.ListMessageAttachments(messageID)
	if err != nil {
		return nil, err
	}
	files := make([]Attachment, 0, len(rows))
	for _, a := range rows {
		files = append(files, Attachment{MimeType: a.MimeType, Path: a.FilePath})
	}
	return files, nil
}

func (s *Service) SetThreadState(threadID int64, state string) error {
	switch state {
	case db.ThreadStateArchived, db.ThreadStateOpen, db.ThreadStateResolved:
	default:
		return errors.Wrapf(ErrThreadStateUnknown, "%q", state)
	}
	return s.store.SetThreadState(threadID, state)
}

func (s *Service) SetThreadStarred(threadID int64, starred bool) error {
	return s.store.SetThreadStarred(threadID, starred)
}

func (s *Service) SetResolver(r Resolver) {
	s.resolver = r
}

func (s *Service) Subscribe(threadID int64) (<-chan struct{}, func() []int64, func()) {
	return s.broker.Subscribe(threadID)
}

func (s *Service) Thread(id int64) (db.Thread, error) {
	return s.store.GetThread(id)
}

func (s *Service) Threads() ([]db.Thread, error) {
	return s.store.ListThreads(db.ThreadKindChat)
}

func (s *Service) View(threadID int64) (ThreadView, error) {
	thread, err := s.store.GetThread(threadID)
	if err != nil {
		return ThreadView{}, err
	}
	msgs, err := s.store.ListThreadMessages(threadID)
	if err != nil {
		return ThreadView{}, err
	}
	events, err := s.store.ListThreadMessageEvents(threadID)
	if err != nil {
		return ThreadView{}, err
	}
	attachments, err := s.store.ListThreadAttachments(threadID)
	if err != nil {
		return ThreadView{}, err
	}
	view := ThreadView{
		Attachments: map[int64][]db.Attachment{},
		Forms:       map[int64]elicit.Prompt{},
		Messages:    msgs,
		Parts:       map[int64][]MessagePart{},
		Thread:      thread,
	}
	for _, a := range attachments {
		if a.MessageID != nil {
			view.Attachments[*a.MessageID] = append(view.Attachments[*a.MessageID], a)
		}
	}
	byMessage := map[int64][]db.MessageEvent{}
	for _, ev := range events {
		byMessage[ev.MessageID] = append(byMessage[ev.MessageID], ev)
	}
	for _, m := range msgs {
		if m.Status == db.MessageStatusStreaming {
			view.Streaming = true
		}
		if m.Role != db.MessageRoleAssistant {
			continue
		}
		collectEvents(&view, m, byMessage[m.ID])
	}
	return view, nil
}

func collectEvents(view *ThreadView, m db.Message, events []db.MessageEvent) {
	if parts := foldParts(events, m.Status == db.MessageStatusStreaming); len(parts) > 0 {
		view.Parts[m.ID] = parts
	} else if m.Body != "" {
		view.Parts[m.ID] = []MessagePart{{Kind: PartText, Text: m.Body}}
	}
	prompts := []elicit.Prompt{}
	resolved := map[string]bool{}
	for _, ev := range events {
		switch ev.Type {
		case EventElicitation:
			var p elicit.Prompt
			if err := json.Unmarshal([]byte(ev.Data), &p); err == nil {
				prompts = append(prompts, p)
			}
		case EventElicitationResult:
			var r elicitationResult
			if err := json.Unmarshal([]byte(ev.Data), &r); err == nil {
				resolved[r.ElicitationID] = true
			}
		}
	}
	if m.Status != db.MessageStatusStreaming {
		return
	}
	for _, p := range prompts {
		if !resolved[p.ElicitationID] {
			view.Forms[m.ID] = p
			return
		}
	}
}

func (s *Service) emit(threadID, messageID int64, ev Event) {
	if _, err := s.store.AddMessageEvent(messageID, ev.Type, ev.Data); err != nil {
		s.log.Error("chat.event", "error", err.Error())
	}
	switch ev.Type {
	case EventAnchor:
		if err := s.store.SetThreadAnchor(threadID, ev.Data); err != nil {
			s.log.Error("chat.anchor", "error", err.Error())
		}
	case EventRunner:
		if err := s.store.SetMessageMeta(messageID, ev.Data); err != nil {
			s.log.Error("chat.runner", "error", err.Error())
		}
	case EventDelta:
		var delta struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &delta); err == nil && delta.Text != "" {
			if err := s.store.AppendMessageBody(messageID, delta.Text); err != nil {
				s.log.Error("chat.append", "error", err.Error())
			}
		}
	}
	s.broker.Publish(threadID, messageID)
}

func (s *Service) resolveAgent(thread db.Thread) (Agent, error) {
	name := s.AgentName(thread)
	agent, ok := s.agents[name]
	if !ok {
		return nil, errors.Wrapf(ErrAgentUnknown, "%q", name)
	}
	return agent, nil
}

func (s *Service) run(agent Agent, thread db.Thread, turn Turn) {
	defer s.wg.Done()

	anchor, err := agent.Run(s.ctx, turn, func(ev Event) {
		s.emit(thread.ID, turn.MessageID, ev)
	})

	if errors.Is(err, ErrTurnPending) {
		s.log.Info("chat.pending", "message", turn.MessageID, "thread", thread.ID)
		return
	}

	status := db.MessageStatusComplete
	if err != nil {
		status = db.MessageStatusError
		s.log.Error("chat.run", "error", err.Error(), "message", turn.MessageID, "thread", thread.ID)
		s.emit(thread.ID, turn.MessageID, Event{Data: errorData(err), Type: EventError})
	}
	if _, err := s.store.FinishMessage(turn.MessageID, status); err != nil {
		s.log.Error("chat.finish", "error", err.Error())
	}
	if anchor != "" && anchor != thread.Anchor {
		if err := s.store.SetThreadAnchor(thread.ID, anchor); err != nil {
			s.log.Error("chat.anchor", "error", err.Error())
		}
	}
	if err := s.store.TouchThread(thread.ID); err != nil {
		s.log.Error("chat.touch", "error", err.Error())
	}
	s.broker.Publish(thread.ID, turn.MessageID)
}

type elicitationResult struct {
	Action        string `json:"action"`
	Content       string `json:"content,omitempty"`
	ElicitationID string `json:"elicitationId"`
}

func AgentName(a Agent) string {
	name := a.Author()
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func errorData(err error) string {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(data)
}

func truncate(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:n])) + "…"
}
