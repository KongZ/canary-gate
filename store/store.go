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
