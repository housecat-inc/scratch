package testkit

import "testing"

type Step[H any] func(t *T, h H)

type Scenario[H any] struct {
	Name  string
	Steps []Step[H]
}

func (s Scenario[H]) name() string { return s.Name }

func RunSteps[H any](t *testing.T, scenarios []Scenario[H], setup func(t *T) H, opts ...Option) {
	t.Helper()
	each(t, scenarios, Scenario[H].name, func(tk *T, s Scenario[H]) {
		h := setup(tk)
		for _, step := range s.Steps {
			step(tk, h)
		}
	}, opts...)
}
