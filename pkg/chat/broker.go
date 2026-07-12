package chat

import "sync"

const StructuralUpdate int64 = 0

type Broker struct {
	mu   sync.Mutex
	next int
	subs map[int64]map[int]*subscription
}

type subscription struct {
	dirty  map[int64]bool
	signal chan struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: map[int64]map[int]*subscription{}}
}

func (b *Broker) Publish(threadID, messageID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subs[threadID] {
		sub.dirty[messageID] = true
		select {
		case sub.signal <- struct{}{}:
		default:
		}
	}
}

func (b *Broker) Subscribe(threadID int64) (<-chan struct{}, func() []int64, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[threadID] == nil {
		b.subs[threadID] = map[int]*subscription{}
	}
	id := b.next
	b.next++
	sub := &subscription{
		dirty:  map[int64]bool{},
		signal: make(chan struct{}, 1),
	}
	b.subs[threadID][id] = sub

	take := func() []int64 {
		b.mu.Lock()
		defer b.mu.Unlock()
		ids := make([]int64, 0, len(sub.dirty))
		for messageID := range sub.dirty {
			ids = append(ids, messageID)
		}
		sub.dirty = map[int64]bool{}
		return ids
	}
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[threadID], id)
		if len(b.subs[threadID]) == 0 {
			delete(b.subs, threadID)
		}
	}
	return sub.signal, take, cancel
}
