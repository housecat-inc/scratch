package chat

import "sync"

type Broker struct {
	mu   sync.Mutex
	next int
	subs map[int64]map[int]chan struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: map[int64]map[int]chan struct{}{}}
}

func (b *Broker) Publish(threadID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[threadID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (b *Broker) Subscribe(threadID int64) (<-chan struct{}, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[threadID] == nil {
		b.subs[threadID] = map[int]chan struct{}{}
	}
	id := b.next
	b.next++
	ch := make(chan struct{}, 1)
	b.subs[threadID][id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[threadID], id)
		if len(b.subs[threadID]) == 0 {
			delete(b.subs, threadID)
		}
	}
}
