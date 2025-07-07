package store

import (
	"github.com/KongZ/canary-gate/service"
)

// Canary Gate Store constants when gate is open
const GATE_OPEN = "opened"

// Canary Gate Store constants when gate is closed
const GATE_CLOSE = "closed"

type StoreKey struct {
	Namespace string
	Name      string
	Type      service.HookType
}

type Store interface {
	GateOpen(key StoreKey)
	GateClose(key StoreKey)
	IsGateOpen(key StoreKey) bool
}

func defaultValue(key StoreKey) bool {
	return key.Type != service.HookRollback
}

func defaultText(key StoreKey) string {
	if defaultValue(key) {
		return GATE_OPEN
	}
	return GATE_CLOSE
}

func GateStatus(val bool) string {
	if val {
		return GATE_OPEN
	}
	return GATE_CLOSE
}

func GateBoolStatus(val string) bool {
	return val == GATE_OPEN
}

func (k *StoreKey) String() string {
	return k.Namespace + "/" + k.Name + "=" + string(k.Type)
}
