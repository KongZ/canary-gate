package store

import (
	"fmt"
	"sync"
)

type MemoryStore struct {
	data *sync.Map
}

func NewMemoryStore() (Store, error) {
	store := &MemoryStore{
		data: new(sync.Map),
	}
	return store, nil
}

func (s *MemoryStore) GateOpen(key StoreKey) {
	s.data.Store(s.getKey(key), true)
}

func (s *MemoryStore) GateClose(key StoreKey) {
	s.data.Store(s.getKey(key), false)
}

func (s *MemoryStore) IsGateOpen(key StoreKey) bool {
	val, ok := s.data.LoadOrStore(s.getKey(key), defaultValue(key))
	if ok {
		return val.(bool)
	}
	return defaultValue(key)
}

// StoreKey get store key name
func (s *MemoryStore) getKey(key StoreKey) string {
	return fmt.Sprintf("%s:%s:%s", key.Namespace, key.Name, key.Type)
}
