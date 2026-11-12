package a

import (
	stderrors "errors"
	"fmt"

	"github.com/cockroachdb/errors"
	pkgerrors "github.com/pkg/errors"
)

func bannedUses(err error) error {
	_ = stderrors.Is(err, stderrors.New("sentinel")) // want "use github.com/cockroachdb/errors.Is instead of stderrors.Is" "use github.com/cockroachdb/errors.New instead of stderrors.New"
	return fmt.Errorf("wrap: %w", err)               // want "use github.com/cockroachdb/errors.Errorf instead of fmt.Errorf"
}

func bannedThirdPartyErrors(err error) error {
	return pkgerrors.Wrap(err, "wrap") // want "use github.com/cockroachdb/errors.Wrap instead of pkgerrors.Wrap"
}

func allowedUses(err error) error {
	_ = errors.Is(err, errors.New("sentinel"))
	return errors.Errorf("wrap: %w", err)
}

func allowedFmt() string {
	return fmt.Sprintf("%s", "ok")
}
