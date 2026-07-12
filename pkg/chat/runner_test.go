package chat

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeParser struct {
	completed bool
	lines     []string
	result    error
}

func (p *fakeParser) Completed() bool { return p.completed }

func (p *fakeParser) Result() error { return p.result }

func (p *fakeParser) Parse(line []byte) []Event {
	text := string(line)
	p.lines = append(p.lines, text)
	if strings.HasPrefix(text, "DONE") {
		p.completed = true
	}
	return []Event{DeltaEvent(text)}
}

func runnerTurnMeta(t *testing.T, out, errPath string, pid int) string {
	t.Helper()
	data, err := json.Marshal(messageMeta{Runner: &runnerMeta{Err: errPath, Out: out, PID: pid}})
	require.NoError(t, err)
	return string(data)
}

func TestRunTurnSpawnsAndCompletes(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	events := []Event{}
	parser := &fakeParser{}
	turn := Turn{MessageID: 424242, Prompt: "go"}
	err := runTurn(context.Background(), turn, func(ev Event) { events = append(events, ev) },
		"/bin/sh", []string{"-c", "echo hello; echo DONE"}, t.TempDir(), parser)
	r.NoError(err)

	a.Equal([]string{"hello", "DONE"}, parser.lines)
	r.NotEmpty(events)
	a.Equal(EventRunner, events[0].Type)
	a.True(HasRunner(events[0].Data))

	runner := runnerFromMeta(events[0].Data)
	_, statErr := os.Stat(runner.Out)
	a.True(os.IsNotExist(statErr))
}

func TestRunTurnAttachesToFinishedRunner(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	out := filepath.Join(dir, "turn.out")
	errPath := filepath.Join(dir, "turn.err")
	r.NoError(os.WriteFile(out, []byte("replayed\nDONE\n"), 0o644))
	r.NoError(os.WriteFile(errPath, nil, 0o644))

	parser := &fakeParser{}
	turn := Turn{MessageID: 1, Meta: runnerTurnMeta(t, out, errPath, -1)}
	err := runTurn(context.Background(), turn, func(Event) {}, "/bin/sh", nil, dir, parser)
	r.NoError(err)
	a.Equal([]string{"replayed", "DONE"}, parser.lines)
}

func TestRunTurnInterruptedRunnerFails(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	out := filepath.Join(dir, "turn.out")
	errPath := filepath.Join(dir, "turn.err")
	r.NoError(os.WriteFile(out, []byte("partial\n"), 0o644))
	r.NoError(os.WriteFile(errPath, []byte("killed by restart"), 0o644))

	parser := &fakeParser{}
	turn := Turn{MessageID: 2, Meta: runnerTurnMeta(t, out, errPath, -1)}
	err := runTurn(context.Background(), turn, func(Event) {}, "claude", nil, dir, parser)
	r.Error(err)
	a.Contains(err.Error(), "turn interrupted")
	a.Contains(err.Error(), "killed by restart")
}

func TestRunTurnNoPromptNoRunner(t *testing.T) {
	a := assert.New(t)

	err := runTurn(context.Background(), Turn{MessageID: 3}, func(Event) {}, "claude", nil, t.TempDir(), &fakeParser{})
	a.ErrorContains(err, "no prompt and no runner")
}

func TestHasRunner(t *testing.T) {
	a := assert.New(t)

	a.False(HasRunner(""))
	a.False(HasRunner("{}"))
	a.False(HasRunner(`{"runner":{}}`))
	a.True(HasRunner(`{"runner":{"out":"/tmp/x.out","pid":1}}`))
}

func TestStopRunnerTerminatesProcessGroup(t *testing.T) {
	r := require.New(t)

	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	r.NoError(cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	r.NoError(stopRunner(&runnerMeta{PID: cmd.Process.Pid}))
	r.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return !pidAlive(cmd.Process.Pid)
		}
	}, 5*time.Second, 50*time.Millisecond)
}
