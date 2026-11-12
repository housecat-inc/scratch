package sessions

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestSessionsBrowser(t *testing.T) {
	t.Run("shows the setup checklist inside the tool shell", func(t *testing.T) {
		kit := testkit.New(t)
		fake := fakeDeps{}
		s, err := NewServer(fake.deps())
		kit.R.NoError(err)
		h := testkit.NewHarnessWithT(t, kit, s.Handler())

		h.Load("/setup")
		h.ElementTextContains(".mail-brand-text", "scratch")
		h.ElementTextContains(`[data-slot="nav"]`, "Setup")
		h.ElementTextContains(`[data-slot="nav"]`, "Code review")
		h.ElementAttributeEquals(`a[href="/setup"]`, "aria-current", "page")
		h.ElementTextContains("#card-install", "Install")
		h.ElementTextContains("#card-login", "Sign in")
		h.ElementTextContains("#card-configure", "Configure")
	})
}
