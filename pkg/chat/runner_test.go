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

func TestRunTurn(t *testing.T) {
	t.Run("spawns a runner, parses its output and removes the log", func(t *testing.T) {
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
	})

	t.Run("replays the output of a finished runner", func(t *testing.T) {
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
	})

	t.Run("fails with the stderr tail when the runner was interrupted", func(t *testing.T) {
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
	})

	t.Run("fails without a prompt or a runner", func(t *testing.T) {
		a := assert.New(t)

		err := runTurn(context.Background(), Turn{MessageID: 3}, func(Event) {}, "claude", nil, t.TempDir(), &fakeParser{})
		a.ErrorContains(err, "no prompt and no runner")
	})
}

func TestHasRunner(t *testing.T) {
	tests := []struct {
		meta string
		name string
		want bool
	}{
		{meta: "", name: "is false for empty meta", want: false},
		{meta: "{}", name: "is false without a runner", want: false},
		{meta: `{"runner":{}}`, name: "is false for a runner without an output log", want: false},
		{meta: `{"runner":{"out":"/tmp/x.out","pid":1}}`, name: "is true for a runner with an output log", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			a.Equal(tt.want, HasRunner(tt.meta))
		})
	}
}

func TestStopRunner(t *testing.T) {
	t.Run("terminates the process group", func(t *testing.T) {
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
	})
}
