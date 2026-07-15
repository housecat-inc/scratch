package flow

import (
	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

func (e *Engine) Deliver(workflowID, topic, idempotencyKey string, reply elicit.Reply) error {
	if _, _, err := e.workflow(workflowID); err != nil {
		return err
	}
	var opts []dbos.SendOption
	if idempotencyKey != "" {
		opts = append(opts, dbos.WithIdempotencyKey(idempotencyKey))
	}
	return errors.Wrap(dbos.Send(e.ctx, workflowID, reply, topic, opts...), "deliver elicitation")
}
