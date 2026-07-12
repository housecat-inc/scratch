package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
)

const runnerPollInterval = 150 * time.Millisecond

type lineParser interface {
	Completed() bool
	Parse(line []byte) []Event
	Result() error
}

type runnerMeta struct {
	Err string `json:"err"`
	Out string `json:"out"`
	PID int    `json:"pid"`
}

type messageMeta struct {
	Runner *runnerMeta `json:"runner,omitempty"`
}

func HasRunner(meta string) bool {
	return runnerFromMeta(meta) != nil
}

func runnerFromMeta(meta string) *runnerMeta {
	var m messageMeta
	if json.Unmarshal([]byte(meta), &m) != nil {
		return nil
	}
	if m.Runner == nil || m.Runner.Out == "" {
		return nil
	}
	return m.Runner
}

func runTurn(ctx context.Context, turn Turn, emit func(Event), bin string, args []string, dir string, parser lineParser) error {
	runner := runnerFromMeta(turn.Meta)
	if runner == nil {
		if turn.Prompt == "" {
			return errors.Newf("%s: no prompt and no runner to attach", bin)
		}
		spawned, err := spawnRunner(bin, args, dir, turn.MessageID)
		if err != nil {
			return err
		}
		runner = spawned
		data, err := json.Marshal(messageMeta{Runner: runner})
		if err != nil {
			return errors.Wrap(err, "marshal runner meta")
		}
		emit(Event{Data: string(data), Type: EventRunner})
	}

	out, err := os.Open(runner.Out)
	if err != nil {
		return errors.Wrapf(err, "%s: open turn output", bin)
	}
	defer out.Close()

	buf := []byte{}
	chunk := make([]byte, 64*1024)
	for {
		n, _ := out.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			for {
				line, rest, found := bytes.Cut(buf, []byte("\n"))
				if !found {
					break
				}
				buf = rest
				for _, ev := range parser.Parse(line) {
					emit(ev)
				}
			}
			continue
		}
		if parser.Completed() {
			break
		}
		if !pidAlive(runner.PID) {
			if len(bytes.TrimSpace(buf)) > 0 {
				for _, ev := range parser.Parse(buf) {
					emit(ev)
				}
			}
			break
		}
		select {
		case <-ctx.Done():
			return ErrTurnPending
		case <-time.After(runnerPollInterval):
		}
	}

	if !parser.Completed() {
		return errors.Newf("%s: turn interrupted: %s", bin, tailOf(runner.Err, 400))
	}
	if err := parser.Result(); err != nil {
		return err
	}
	_ = os.Remove(runner.Out)
	_ = os.Remove(runner.Err)
	return nil
}

func spawnRunner(bin string, args []string, dir string, messageID int64) (*runnerMeta, error) {
	base, err := turnsDir()
	if err != nil {
		return nil, err
	}
	name := strconv.FormatInt(messageID, 10)
	runner := &runnerMeta{
		Err: filepath.Join(base, name+".err"),
		Out: filepath.Join(base, name+".out"),
	}
	out, err := os.OpenFile(runner.Out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "create turn output")
	}
	defer out.Close()
	errFile, err := os.OpenFile(runner.Err, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "create turn stderr")
	}
	defer errFile.Close()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stderr = errFile
	cmd.Stdout = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "start %s", bin)
	}
	runner.PID = cmd.Process.Pid
	go cmd.Wait()
	return runner, nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func tailOf(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(bytes.TrimSpace(data))
	if len(text) > n {
		text = text[len(text)-n:]
	}
	return text
}

func turnsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "user home dir")
	}
	dir := filepath.Join(home, ".config", "scratch", "turns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.Wrap(err, "create turns dir")
	}
	return dir, nil
}
