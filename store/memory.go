/*
Copyright 2025 The canary-gate authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/KongZ/canary-gate/service"
)

type MemoryStore struct {
	data *sync.Map
}

// NewMemoryStore creates a new MemoryStore instance.
// MemoryStore uses an in-memory map to store gate states.
// It is suitable for testing or scenarios where persistence is not required.
func NewMemoryStore() (Store, error) {
	store := &MemoryStore{
		data: new(sync.Map),
	}
	return store, nil
}

func (s *MemoryStore) GateOpen(key StoreKey) {
	s.data.Store(s.getKey(key), true)
	s.UpdateEvent(context.Background(), key, "Updated", fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), GATE_OPEN))
}

func (s *MemoryStore) GateClose(key StoreKey) {
	s.data.Store(s.getKey(key), false)
	s.UpdateEvent(context.Background(), key, "Updated", fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), GATE_CLOSE))
}

func (s *MemoryStore) IsGateOpen(key StoreKey) bool {
	val, ok := s.data.LoadOrStore(s.getKey(key), defaultValue(key))
	if ok {
		return val.(bool)
	}
	return defaultValue(key)
}

func (s *MemoryStore) UpdateEvent(ctx context.Context, key StoreKey, status string, message string) {
	s.data.Store(s.getEventKey(key), message)
}

func (s *MemoryStore) Shutdown() error {
	return nil
}

// StoreKey get store key name
func (s *MemoryStore) getKey(key StoreKey) string {
	return fmt.Sprintf("%s:%s:%s", key.Namespace, key.Name, key.Type)
}

// StoreKey get store key name
func (s *MemoryStore) getEventKey(key StoreKey) string {
	return fmt.Sprintf("%s:%s:%s", key.Namespace, key.Name, string(service.HookEvent))
}

func (s *MemoryStore) GetLastEvent(ctx context.Context, key StoreKey) string {
	if v, ok := s.data.Load(s.getEventKey(key)); ok {
		return v.(string)
	}
	return ""
}
