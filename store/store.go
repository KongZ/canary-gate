package store

import (
	"github.com/KongZ/canary-gate/service"
)

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
		return "open"
	}
	return "close"
}

func (k *StoreKey) String() string {
	return k.Namespace + ":" + k.Name + ":" + string(k.Type)
}
