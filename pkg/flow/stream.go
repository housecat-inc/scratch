package flow

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	defaultStageStep = time.Second
	streamKey        = "log"
)

var streamStages = []string{"Connecting", "Downloading", "Extracting", "Verifying"}

type StreamInput struct{}

func (e *Engine) stream(ctx dbos.DBOSContext, _ StreamInput) (string, error) {
	for i, stage := range streamStages {
		line := fmt.Sprintf("[%d/%d] %s…", i+1, len(streamStages), stage)
		if err := dbos.WriteStream(ctx, streamKey, line); err != nil {
			return "", errors.Wrapf(err, "write %q", stage)
		}
		if _, err := dbos.Sleep(ctx, e.stageStep); err != nil {
			return "", errors.Wrap(err, "sleep")
		}
	}
	done := fmt.Sprintf("[%d/%d] Done ✓", len(streamStages), len(streamStages))
	if err := dbos.WriteStream(ctx, streamKey, done); err != nil {
		return "", errors.Wrap(err, "write done")
	}
	if err := dbos.CloseStream(ctx, streamKey); err != nil {
		return "", errors.Wrap(err, "close stream")
	}
	return fmt.Sprintf("Streamed %d log lines", len(streamStages)+1), nil
}
