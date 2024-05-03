package store

import "sync"

type MemoryStore struct {
	data *sync.Map
}

func NewMemoryStore() (Store, error) {
	store := &MemoryStore{
		data: new(sync.Map),
	}
	return store, nil
}

func (s *MemoryStore) GateOpen(key string) {
	s.data.Store(key, true)
}

func (s *MemoryStore) GateClose(key string) {
	s.data.Store(key, false)
}

func (s *MemoryStore) IsGateOpen(key string) bool {
	val, ok := s.data.LoadOrStore(key, false)
	if ok {
		return val.(bool)
	}
	return false
}
