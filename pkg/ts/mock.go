package ts

import (
	"sync"
	"time"
)

var (
	mockTime *time.Time
	mu       sync.RWMutex
)

type MockTime struct{}

func (m *MockTime) Advance(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	if mockTime == nil {
		panic("ts: Advance called when no mock time is set; call Mock first and ensure restore has not been invoked")
	}
	t := mockTime.Add(d)
	mockTime = &t
}

func Mock(t time.Time) (*MockTime, func()) {
	mu.Lock()
	mockTime = &t
	mu.Unlock()
	return &MockTime{}, func() {
		mu.Lock()
		mockTime = nil
		mu.Unlock()
	}
}
