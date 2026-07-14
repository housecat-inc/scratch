package flow

import (
	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	deployVersion   = "v1.4.2"
	eventStatusKey  = "status"
	eventVersionKey = "version"
)

var deployStages = []string{"Building", "Testing", "Publishing", "Promoting"}

type DeployInput struct{}

func (e *Engine) deploy(ctx dbos.DBOSContext, _ DeployInput) (string, error) {
	if err := dbos.SetEvent(ctx, eventVersionKey, deployVersion); err != nil {
		return "", errors.Wrap(err, "publish version")
	}
	for _, stage := range deployStages {
		if err := dbos.SetEvent(ctx, eventStatusKey, stage); err != nil {
			return "", errors.Wrapf(err, "publish %q", stage)
		}
		if _, err := dbos.Sleep(ctx, e.stageStep); err != nil {
			return "", errors.Wrap(err, "sleep")
		}
	}
	if err := dbos.SetEvent(ctx, eventStatusKey, "Live"); err != nil {
		return "", errors.Wrap(err, "publish live")
	}
	return "Deployed " + deployVersion, nil
}
