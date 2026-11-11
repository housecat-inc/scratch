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
		h.ElementTextContains(".mail-brand", "scratch")
		h.ElementTextContains(".mail-labels", "Setup")
		h.ElementTextContains(".mail-labels", "Code review")
		h.ElementAttributeContains(`a[href="/setup"]`, "class", "active")
		h.ElementTextContains("#card-install", "Install")
		h.ElementTextContains("#card-login", "Sign in")
		h.ElementTextContains("#card-configure", "Configure")
	})
}
