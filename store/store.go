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
	"github.com/KongZ/canary-gate/service"
)

// Canary Gate Store constants when gate is open
const GATE_OPEN = "opened"

// Canary Gate Store constants when gate is closed
const GATE_CLOSE = "closed"

// StoreKey represents a unique key for a gate in the store.
type StoreKey struct {
	// Namespace is the namespace of the gate.
	Namespace string
	// Name is the name of the gate.
	Name string
	// Type is the type of the gate, which corresponds to a specific hook type.
	Type service.HookType
}

// Store is an interface that defines methods for managing gate states.
type Store interface {
	GateOpen(key StoreKey)
	GateClose(key StoreKey)
	IsGateOpen(key StoreKey) bool
	Shutdown() error
}

// defaultValue returns the default gate status based on the hook type.
func defaultValue(key StoreKey) bool {
	return key.Type != service.HookRollback
}

// defaultText returns the default text representation of the gate status based on the hook type.
func defaultText(key StoreKey) string {
	if defaultValue(key) {
		return GATE_OPEN
	}
	return GATE_CLOSE
}

// GateStatus converts a boolean value to a string representation of the gate status.
func GateStatus(val bool) string {
	if val {
		return GATE_OPEN
	}
	return GATE_CLOSE
}

// GateBoolStatus converts a string representation of the gate status to a boolean value.
func GateBoolStatus(val string) bool {
	return val == GATE_OPEN
}

// String returns a string representation of the StoreKey.
func (k *StoreKey) String() string {
	return k.Namespace + "/" + k.Name + "=" + string(k.Type)
}
