package a

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	tk "github.com/housecat-inc/scratch/testkit/v2" // want `import of github.com/housecat-inc/scratch/testkit/v2 is banned; write stdlib table-driven tests`
)

func TestBanned(t *testing.T) {
	t.Run("uses runners", func(t *testing.T) {
		testkit.RunCases() // want `testkit.RunCases is banned; write stdlib table-driven tests`
		tk.Run()
	})
}
