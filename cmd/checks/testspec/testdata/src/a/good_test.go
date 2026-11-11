package a

import "testing"

func TestGood(t *testing.T) {
	shared := []struct {
		name string
		in   int
	}{
		{name: "accepts zero", in: 0},
		{name: "accepts one", in: 1},
	}

	t.Run("Add", func(t *testing.T) {
		for _, tt := range shared {
			t.Run(tt.name, func(t *testing.T) { _ = tt.in })
		}
	})

	t.Run("when the input is a map", func(t *testing.T) {
		for name, want := range map[string]int{"counts keys": 1, "URL keys stay upper": 2} {
			t.Run(name, func(t *testing.T) { _ = want })
		}
	})

	t.Run("rejects nothing", func(t *testing.T) {})
}

func TestMain(m *testing.M) { m.Run() }

func BenchmarkGood(b *testing.B) {}
