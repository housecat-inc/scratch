package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
)

const (
	titleMaxLen = 80
	userAuthor  = "you"
)

var ErrThreadBusy = errors.New("thread busy")

func IsThreadBusy(err error) bool { return errors.Is(err, ErrThreadBusy) }

type ThreadView struct {
	Messages  []db.Message
	Streaming bool
	Thread    db.Thread
	ToolCalls map[int64][]string
}

type Service struct {
	agent  Agent
	broker *Broker
	cancel context.CancelFunc
	ctx    context.Context
	log    *slog.Logger
	store  db.ThreadStore
	wg     sync.WaitGroup
}

func NewService(store db.ThreadStore, agent Agent, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		agent:  agent,
		broker: NewBroker(),
		cancel: cancel,
		ctx:    ctx,
		log:    log,
		store:  store,
	}
}

func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Service) CreateThread(title string) (db.Thread, error) {
	return s.store.AddThread(db.ThreadKindChat, strings.TrimSpace(title), "")
}

func (s *Service) Send(threadID int64, prompt string) (db.Message, error) {
	thread, err := s.store.GetThread(threadID)
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
	asst, err := s.store.AddMessage(db.NewMessage{
		Author:   s.agent.Author(),
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
	s.broker.Publish(threadID)

	s.wg.Add(1)
	go s.run(thread, asst, prompt)
	return user, nil
}

func (s *Service) Subscribe(threadID int64) (<-chan struct{}, func()) {
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
	view := ThreadView{Messages: msgs, Thread: thread, ToolCalls: map[int64][]string{}}
	for _, m := range msgs {
		if m.Status == db.MessageStatusStreaming {
			view.Streaming = true
		}
		if m.Role != db.MessageRoleAssistant {
			continue
		}
		names, err := s.toolCalls(m.ID)
		if err != nil {
			return ThreadView{}, err
		}
		if len(names) > 0 {
			view.ToolCalls[m.ID] = names
		}
	}
	return view, nil
}

func (s *Service) emit(threadID, messageID int64, ev Event) {
	if _, err := s.store.AddMessageEvent(messageID, ev.Type, ev.Data); err != nil {
		s.log.Error("chat.event", "error", err.Error())
	}
	if ev.Type == EventDelta {
		var delta struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &delta); err == nil && delta.Text != "" {
			if err := s.store.AppendMessageBody(messageID, delta.Text); err != nil {
				s.log.Error("chat.append", "error", err.Error())
			}
		}
	}
	s.broker.Publish(threadID)
}

func (s *Service) run(thread db.Thread, asst db.Message, prompt string) {
	defer s.wg.Done()

	anchor, err := s.agent.Run(s.ctx, thread.Anchor, prompt, func(ev Event) {
		s.emit(thread.ID, asst.ID, ev)
	})

	status := db.MessageStatusComplete
	if err != nil {
		status = db.MessageStatusError
		s.log.Error("chat.run", "error", err.Error(), "message", asst.ID, "thread", thread.ID)
		s.emit(thread.ID, asst.ID, DeltaEvent(err.Error()))
		s.emit(thread.ID, asst.ID, Event{Data: errorData(err), Type: EventError})
	}
	if _, err := s.store.FinishMessage(asst.ID, status); err != nil {
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
	s.broker.Publish(thread.ID)
}

func (s *Service) toolCalls(messageID int64) ([]string, error) {
	events, err := s.store.ListMessageEvents(messageID, 0)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, ev := range events {
		if ev.Type != EventToolCall {
			continue
		}
		var call struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(ev.Data), &call); err == nil && call.Name != "" {
			names = append(names, call.Name)
		}
	}
	return names, nil
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
