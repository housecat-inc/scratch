package flow

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const updateTimeout = 24 * time.Hour

type UpdateInput struct{}

type Confirm struct{}

func (e *Engine) updateClaude(ctx dbos.DBOSContext, _ UpdateInput) (string, error) {
	version, err := act(ctx, "check_version", "Check installed version", "", func(stepCtx context.Context) (string, error) {
		return e.updater.Version(stepCtx)
	})
	if err != nil {
		return "", errors.Wrap(err, "check version")
	}

	_, action, err := elicit.Form(ctx, "confirm", "Update Claude Code to the latest version?", Confirm{}, updateTimeout)
	if err != nil {
		return "", errors.Wrap(err, "confirm update")
	}
	if action != elicit.ActionAccept {
		return "", nil
	}

	result, err := act(ctx, "respond/update", "Update Claude Code", version, func(stepCtx context.Context) (string, error) {
		return e.updater.Update(stepCtx)
	})
	if err != nil {
		return "", errors.Wrap(err, "update claude")
	}
	return result, nil
}

type Updater interface {
	Update(ctx context.Context) (string, error)
	Version(ctx context.Context) (string, error)
}

type UpdaterFuncs struct {
	UpdateFn  func(ctx context.Context) (string, error)
	VersionFn func(ctx context.Context) (string, error)
}

func (u UpdaterFuncs) Update(ctx context.Context) (string, error)  { return u.UpdateFn(ctx) }
func (u UpdaterFuncs) Version(ctx context.Context) (string, error) { return u.VersionFn(ctx) }

func DefaultUpdater() Updater {
	if _, err := exec.LookPath("claude"); err != nil {
		return UpdaterFuncs{
			UpdateFn:  func(context.Context) (string, error) { return "Claude Code is not installed.", nil },
			VersionFn: func(context.Context) (string, error) { return "not installed", nil },
		}
	}
	return claudeUpdater{}
}

type claudeUpdater struct{}

func (claudeUpdater) Version(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "claude", "--version").CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "claude --version: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (claudeUpdater) Update(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "claude", "update").CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return "", errors.Wrapf(err, "claude update: %s", text)
	}
	if text == "" {
		text = "Claude Code is up to date."
	}
	return text, nil
}
