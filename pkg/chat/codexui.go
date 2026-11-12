package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/ui/app"
	"github.com/housecat-inc/scratch/ui/components/chatcomposer"
	codexui "github.com/housecat-inc/scratch/ui/components/codex"
)

const codexRefreshInterval = 3 * time.Second

type codexSend struct {
	MessageID int64
	ThreadID  int64
}

type CodexServer struct {
	clients map[int64]string
	label   string
	mu      sync.Mutex
	sends   map[string]codexSend
	svc     *Service
}

func NewCodexServer(svc *Service) *CodexServer {
	return &CodexServer{clients: map[int64]string{}, sends: map[string]codexSend{}, svc: svc}
}

func (s *CodexServer) WithLabel(label string) *CodexServer {
	s.label = label
	return s
}

func (s *CodexServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /codex", s.handleChat)
	mux.HandleFunc("POST /codex/attachments", s.handleUpload)
	mux.HandleFunc("DELETE /codex/attachments/{attachment}", s.handleDeleteAttachment)
	mux.HandleFunc("GET /codex/attachments/{attachment}", s.handleAttachment)
	mux.HandleFunc("GET /codex/chat/events", s.handleEvents)
	mux.HandleFunc("GET /codex/link-preview", http.NotFound)
	mux.HandleFunc("GET /codex/sends", s.handleSends)
	mux.HandleFunc("POST /codex/threads", s.handleCreate)
	mux.HandleFunc("DELETE /codex/threads/{thread}", s.handleDelete)
	mux.HandleFunc("POST /codex/threads/{thread}/messages", s.handleMessage)
	mux.HandleFunc("GET /codex/threads/{thread}/queue", s.handleQueue)
	mux.HandleFunc("POST /codex/threads/{thread}/turns/{turn}/stop", s.handleStop)
	return mux
}

func (s *CodexServer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(carryChatWriter{ResponseWriter: w, r: r}, r)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			next.ServeHTTP(w, r)
			return
		}
		param, open := r.URL.Query()["chat"]
		selected := ""
		if open && param[0] != "new" {
			selected = param[0]
		}
		in, err := s.chatIn(selected, r.URL.RequestURI())
		if err != nil {
			in = s.newChatIn()
		}
		ctx := app.WithRail(r.Context(), app.Rail{Chat: codexui.Chat(in), Open: open})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type carryChatWriter struct {
	http.ResponseWriter
	r *http.Request
}

func (w carryChatWriter) WriteHeader(status int) {
	if status >= 300 && status < 400 {
		if location := carryChat(w.r, w.Header().Get("Location")); location != "" {
			w.Header().Set("Location", location)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func carryChat(r *http.Request, location string) string {
	referer, err := url.Parse(r.Referer())
	if err != nil || referer.Host != r.Host {
		return ""
	}
	chat := referer.Query().Get("chat")
	to, err := url.Parse(location)
	if chat == "" || err != nil || (to.Host != "" && to.Host != r.Host) || to.Query().Has("chat") {
		return ""
	}
	q := to.Query()
	q.Set("chat", chat)
	to.RawQuery = q.Encode()
	return to.String()
}

func (s *CodexServer) handleChat(w http.ResponseWriter, r *http.Request) {
	selected := r.URL.Query().Get("chat")
	if selected == "new" {
		selected = ""
	}
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/?chat="+url.QueryEscape(cmpOr(selected, "new")), http.StatusSeeOther)
		return
	}
	in, err := s.chatIn(selected, r.Header.Get("HX-Current-URL"))
	if err != nil {
		in = s.newChatIn()
	}
	w.Header().Add("Vary", "HX-Request")
	w.Header().Set("HX-Push-Url", "false")
	render(w, r, codexui.Chat(in))
}

func (s *CodexServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	clientID := r.FormValue("client_id")
	if send, ok := s.sent(clientID); ok {
		s.writeOutgoing(w, r, clientID, send)
		return
	}
	if parent := r.FormValue("client_thread_id"); parent != "" && parent != clientID {
		send, ok := s.sent(parent)
		if !ok {
			s.writeSendError(w, r, http.StatusConflict, "The chat isn't started yet. Resend once it is.")
			return
		}
		s.send(w, r, send.ThreadID, clientID)
		return
	}
	agent, model := ParseProviderModel(r.FormValue("model"))
	if agent == "" {
		agent = s.svc.defaultName
	}
	thread, err := s.svc.CreateThreadWithLabel(agent, NormalizeModel(agent, model), s.label, codexTitle(r.FormValue("prompt")))
	if err != nil {
		s.writeSendError(w, r, http.StatusInternalServerError, "Unable to send. Please try again.")
		return
	}
	s.send(w, r, thread.ID, clientID)
}

func (s *CodexServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	clientID := r.FormValue("client_id")
	if send, ok := s.sent(clientID); ok {
		s.writeOutgoing(w, r, clientID, send)
		return
	}
	threadID, err := strconv.ParseInt(r.PathValue("thread"), 10, 64)
	if err != nil {
		s.writeSendError(w, r, http.StatusBadRequest, "Choose a chat to send to.")
		return
	}
	s.send(w, r, threadID, clientID)
}

func (s *CodexServer) send(w http.ResponseWriter, r *http.Request, threadID int64, clientID string) {
	attachmentIDs, err := parseAttachmentIDs(r.FormValue("attachments"))
	if err != nil {
		s.writeSendError(w, r, http.StatusBadRequest, "Remove the broken attachment and resend.")
		return
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt == "" && len(attachmentIDs) > 0 {
		prompt = attachmentPrompt
	}
	if prompt == "" {
		s.writeSendError(w, r, http.StatusBadRequest, "Write a message first.")
		return
	}
	msg, err := s.svc.Send(threadID, prompt, attachmentIDs...)
	if db.IsAttachmentNotFound(err) {
		s.writeSendError(w, r, http.StatusBadRequest, "An attachment is no longer available. Remove it and resend.")
		return
	}
	if err != nil {
		s.writeSendError(w, r, http.StatusInternalServerError, "Unable to send. Please try again.")
		return
	}
	send := codexSend{MessageID: msg.ID, ThreadID: threadID}
	if clientID != "" {
		s.mu.Lock()
		s.sends[clientID] = send
		s.clients[msg.ID] = clientID
		s.mu.Unlock()
	}
	s.svc.Publish(threadID)
	s.writeOutgoing(w, r, clientID, send)
}

func parseAttachmentIDs(raw string) ([]int64, error) {
	ids := []int64{}
	for field := range strings.SplitSeq(raw, ",") {
		if field = strings.TrimSpace(field); field == "" {
			continue
		}
		id, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "parse attachment id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *CodexServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, attachmentMaxBytes+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, ErrAttachmentTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	attachment, err := s.svc.AddAttachment(0, header.Filename, header.Header.Get("Content-Type"), file)
	if errors.Is(err, ErrAttachmentTooLarge) {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		http.Error(w, "Unable to attach the file. Please try again.", http.StatusInternalServerError)
		return
	}
	render(w, r, chatcomposer.AttachmentChip(composerAttachment(attachment), true))
}

func (s *CodexServer) handleAttachment(w http.ResponseWriter, r *http.Request) {
	attachment, ok := s.attachment(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": attachment.Name}))
	http.ServeFile(w, r, attachment.FilePath)
}

func (s *CodexServer) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	attachment, ok := s.attachment(w, r)
	if !ok {
		return
	}
	err := s.svc.DeleteDraftAttachment(attachment.ID)
	if db.IsAttachmentNotFound(err) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Unable to remove the attachment. Please try again.", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *CodexServer) attachment(w http.ResponseWriter, r *http.Request) (db.Attachment, bool) {
	id, err := strconv.ParseInt(r.PathValue("attachment"), 10, 64)
	if err != nil {
		http.Error(w, "invalid attachment id", http.StatusBadRequest)
		return db.Attachment{}, false
	}
	attachment, err := s.svc.Attachment(id)
	if db.IsAttachmentNotFound(err) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return db.Attachment{}, false
	}
	if err != nil {
		http.Error(w, "Unable to load the attachment.", http.StatusInternalServerError)
		return db.Attachment{}, false
	}
	return attachment, true
}

func composerAttachment(a db.Attachment) chatcomposer.Attachment {
	return chatcomposer.Attachment{
		ID:    itoa(a.ID),
		Image: strings.HasPrefix(a.MimeType, "image/"),
		Name:  a.Name,
		URL:   codexui.Path + "/attachments/" + itoa(a.ID),
	}
}

func (s *CodexServer) sent(clientID string) (codexSend, bool) {
	if clientID == "" {
		return codexSend{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	send, ok := s.sends[clientID]
	return send, ok
}

func (s *CodexServer) writeOutgoing(w http.ResponseWriter, r *http.Request, clientID string, send codexSend) {
	render(w, r, codexui.OutgoingState(codexui.OutgoingStateIn{
		ClientID: clientID,
		ThreadID: itoa(send.ThreadID),
		TurnID:   itoa(send.MessageID),
	}))
}

func (s *CodexServer) writeSendError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = codexui.SendError(message).Render(r.Context(), w)
}

func (s *CodexServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	threadID, err := strconv.ParseInt(r.URL.Query().Get("thread"), 10, 64)
	if err != nil {
		http.Error(w, "Select a chat.", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")

	updates, take, cancel := s.svc.Subscribe(threadID)
	defer cancel()
	tick := time.NewTicker(codexRefreshInterval)
	defer tick.Stop()

	page := r.Header.Get("HX-Current-URL")
	last := ""
	for {
		frame, err := s.frame(r.Context(), threadID, page)
		if err != nil {
			return
		}
		if frame != last {
			writeEventData(w, frame)
			last = frame
		} else {
			fmt.Fprint(w, ": heartbeat\n\n")
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			take()
		case <-tick.C:
		}
	}
}

func (s *CodexServer) frame(ctx context.Context, threadID int64, page string) (string, error) {
	in, err := s.chatIn(itoa(threadID), page)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := codexui.ChatContents(in).Render(ctx, &buf); err != nil {
		return "", errors.Wrap(err, "render chat")
	}
	return buf.String(), nil
}

func writeEventData(w http.ResponseWriter, html string) {
	html = strings.ReplaceAll(strings.ReplaceAll(html, "\r\n", "\n"), "\r", "\n")
	for line := range strings.SplitSeq(html, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}

func (s *CodexServer) handleQueue(w http.ResponseWriter, r *http.Request) {
	render(w, r, codexui.QueueContents(nil))
}

func (s *CodexServer) handleSends(w http.ResponseWriter, r *http.Request) {
	ids := r.URL.Query()["id"]
	if len(ids) == 0 || len(ids) > 50 {
		http.Error(w, "Ask about 1 to 50 messages.", http.StatusBadRequest)
		return
	}
	out := map[string]map[string]string{}
	for _, id := range ids {
		if send, ok := s.sent(id); ok {
			out[id] = map[string]string{"status": "sent", "threadId": itoa(send.ThreadID), "turnId": itoa(send.MessageID)}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *CodexServer) handleStop(w http.ResponseWriter, r *http.Request) {
	threadID, err := strconv.ParseInt(r.PathValue("thread"), 10, 64)
	if err != nil {
		s.writeSendError(w, r, http.StatusBadRequest, "Choose a chat to stop.")
		return
	}
	if err := s.svc.StopThread(threadID); err != nil {
		s.writeSendError(w, r, http.StatusInternalServerError, "Unable to stop this turn. Please try again.")
		return
	}
	render(w, r, codexui.StopResult("Stopping…"))
}

func (s *CodexServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	threadID, err := strconv.ParseInt(r.PathValue("thread"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat.", http.StatusBadRequest)
		return
	}
	if err := s.svc.TrashThread(threadID); err != nil {
		http.Error(w, "Unable to delete this conversation. Please try again.", http.StatusInternalServerError)
		return
	}
	event, _ := json.Marshal(map[string]map[string]string{codexui.Namespace + ":thread-deleted": {"threadID": itoa(threadID)}})
	w.Header().Set("HX-Trigger", string(event))
	w.WriteHeader(http.StatusNoContent)
}

func (s *CodexServer) newChatIn() codexui.ChatIn {
	in := s.baseChatIn()
	in.SubmitURL = codexui.Path + "/threads?surface=mail"
	for _, option := range ProviderModelOptions(s.svc.AgentNames()) {
		in.ModelOptions = append(in.ModelOptions, codexui.ModelOption{Label: option.Label, Value: option.Value})
	}
	if len(in.ModelOptions) > 0 {
		in.Model = in.ModelOptions[0].Value
		for _, option := range in.ModelOptions {
			if strings.HasPrefix(option.Value, s.svc.defaultName+":") {
				in.Model = option.Value
				break
			}
		}
	}
	return in
}

func (s *CodexServer) baseChatIn() codexui.ChatIn {
	return codexui.ChatIn{
		AttachURL:         codexui.Path + "/attachments",
		EventsURLTemplate: codexui.Path + "/chat/events?" + url.Values{"surface": {"mail"}, "thread": {codexui.ThreadPlaceholder}}.Encode(),
		History:           codexui.HistoryPagesApart,
		ID:                codexui.MailContentID,
		PromptPlaceholder: "Ask anything...",
		QueueURLTemplate:  codexui.Path + "/threads/" + codexui.ThreadPlaceholder + "/queue",
		SendsURL:          codexui.Path + "/sends",
		SubmitURLTemplate: codexui.Path + "/threads/" + codexui.ThreadPlaceholder + "/messages?surface=mail",
		User:              "local",
	}
}

func (s *CodexServer) chatIn(selected, page string) (codexui.ChatIn, error) {
	if selected == "" {
		return s.newChatIn(), nil
	}
	threadID, err := strconv.ParseInt(selected, 10, 64)
	if err != nil {
		return codexui.ChatIn{}, errors.Wrap(err, "parse thread id")
	}
	view, err := s.svc.View(threadID)
	if err != nil {
		return codexui.ChatIn{}, err
	}
	if view.Thread.TrashedAt != nil {
		return codexui.ChatIn{}, errors.New("chat deleted")
	}
	in := s.baseChatIn()
	in.DeleteURL = codexui.Path + "/threads/" + selected + "?surface=mail"
	in.EventsURL = codexui.Path + "/chat/events?" + url.Values{"surface": {"mail"}, "thread": {selected}}.Encode()
	in.Model = s.svc.AgentName(view.Thread)
	in.NewChatURL = newChatURL(page)
	in.QueueURL = codexui.Path + "/threads/" + selected + "/queue"
	in.SelectedID = selected
	in.Status = "idle"
	in.SubmitURL = codexui.Path + "/threads/" + selected + "/messages?surface=mail"
	in.Title = cmpOr(view.Thread.Title, "Chat")
	in.Turns = s.turns(view)
	if view.Streaming {
		in.Status = "active"
		if n := len(in.Turns); n > 0 {
			_, turn, _ := strings.Cut(in.Turns[n-1].ID, "/")
			in.StopURL = codexui.StopPath(selected, turn)
		}
	}
	return in, nil
}

func (s *CodexServer) turns(view ThreadView) []codexui.ChatTurn {
	thread := itoa(view.Thread.ID)
	turns := []codexui.ChatTurn{}
	for _, m := range view.Messages {
		switch m.Role {
		case db.MessageRoleUser:
			s.mu.Lock()
			clientID := s.clients[m.ID]
			s.mu.Unlock()
			turns = append(turns, codexui.ChatTurn{
				ID:        thread + "/" + itoa(m.ID),
				Prompt:    []codexui.ChatItem{{Attachments: composerAttachments(view.Attachments[m.ID]), ClientID: clientID, ID: thread + "/" + itoa(m.ID) + "/prompt", Kind: "message", Text: m.Body, Title: "You"}},
				StartedAt: m.CreatedAt.Time,
				Status:    "completed",
			})
		case db.MessageRoleAssistant:
			if len(turns) == 0 {
				turns = append(turns, codexui.ChatTurn{ID: thread + "/" + itoa(m.ID), StartedAt: m.CreatedAt.Time})
			}
			turn := &turns[len(turns)-1]
			turn.Status = turnStatus(m.Status)
			if m.Status == db.MessageStatusError {
				turn.Error = cmpOr(m.Body, "The turn failed.")
			}
			for i, part := range view.Parts[m.ID] {
				item := partItem(part)
				item.ID = turn.ID + "/" + itoa(m.ID) + "-" + strconv.Itoa(i)
				if part.Kind == PartText {
					turn.Answer = append(turn.Answer, item)
				} else {
					turn.Activity = append(turn.Activity, item)
				}
			}
		}
	}
	return turns
}

func composerAttachments(attachments []db.Attachment) []chatcomposer.Attachment {
	out := make([]chatcomposer.Attachment, 0, len(attachments))
	for _, a := range attachments {
		out = append(out, composerAttachment(a))
	}
	return out
}

func turnStatus(status string) string {
	switch status {
	case db.MessageStatusStreaming:
		return "inProgress"
	case db.MessageStatusError:
		return "failed"
	}
	return "completed"
}

func partItem(part MessagePart) codexui.ChatItem {
	switch part.Kind {
	case PartThinking:
		return codexui.ChatItem{Kind: "thinking", Text: part.Text, Title: "Thinking"}
	case PartPlan:
		lines := make([]string, 0, len(part.Plan))
		for _, entry := range part.Plan {
			mark := "- [ ] "
			if entry.Status == "completed" {
				mark = "- [x] "
			}
			lines = append(lines, mark+entry.Content)
		}
		return codexui.ChatItem{Kind: "plan", Text: strings.Join(lines, "\n"), Title: "Plan"}
	case PartTool:
		item := codexui.ChatItem{Kind: "tool", Title: "Used tool"}
		if part.Tool != nil {
			item.Title = cmpOr(part.Tool.Title, item.Title)
			item.Status = toolStatus(part.Tool.Status)
			if part.Tool.Detail != "" {
				item.Blocks = []codexui.DetailBlock{{Kind: "code", Text: part.Tool.Detail, Title: "Output"}}
			}
		}
		return item
	case PartError:
		return codexui.ChatItem{Kind: "notice", Text: part.Text, Title: "Error"}
	}
	return codexui.ChatItem{Kind: "message", Text: part.Text, Title: "Codex"}
}

func toolStatus(status string) string {
	switch status {
	case "in_progress", "pending":
		return "inProgress"
	}
	return status
}

func newChatURL(page string) string {
	u, err := url.Parse(page)
	if err != nil || page == "" {
		return "/?chat=new"
	}
	q := u.Query()
	q.Set("chat", "new")
	return u.Path + "?" + q.Encode()
}

func codexTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if line, _, ok := strings.Cut(prompt, "\n"); ok {
		prompt = line
	}
	if len(prompt) > 60 {
		prompt = prompt[:60] + "…"
	}
	return prompt
}

func cmpOr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}
