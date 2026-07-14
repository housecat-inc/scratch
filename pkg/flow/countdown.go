package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	defaultCountdownStep  = 2 * time.Second
	defaultCountdownTicks = 3
)

type CountdownInput struct{}

func (e *Engine) countdown(ctx dbos.DBOSContext, _ CountdownInput) (string, error) {
	for n := e.countdownTicks; n >= 1; n-- {
		tick := n
		if _, err := act(ctx, fmt.Sprintf("respond/tick-%d", tick), fmt.Sprintf("T-minus %d", tick), "", func(context.Context) (string, error) {
			return fmt.Sprintf("%d…", tick), nil
		}); err != nil {
			return "", errors.Wrapf(err, "tick %d", tick)
		}
		if _, err := dbos.Sleep(ctx, e.countdownStep); err != nil {
			return "", errors.Wrapf(err, "sleep before tick %d", tick-1)
		}
	}
	const liftoff = "🚀 Liftoff!"
	if _, err := act(ctx, "respond/liftoff", "Liftoff", "", func(context.Context) (string, error) {
		return liftoff, nil
	}); err != nil {
		return "", errors.Wrap(err, "liftoff")
	}
	return liftoff, nil
}
