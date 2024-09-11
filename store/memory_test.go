package store

import (
	"testing"
)

func TestMemoryGate(t *testing.T) {
	for _, v := range typeCases {
		serviceType := v.serviceType
		sk := StoreKey{
			Namespace: "canary-ns",
			Name:      "test-canary",
			Type:      serviceType,
		}
		// Tests
		store, err := NewMemoryStore()
		if err != nil {
			t.Error(err)
		}
		result := store.IsGateOpen(sk)
		if v.expectedInit != result {
			t.Fatalf("[%s] [default] gate expected %v found %v", serviceType, v.expectedInit, result)
		}
		// close gate
		store.GateClose(sk)
		result = store.IsGateOpen(sk)
		if v.expectedAfterClose != result {
			t.Fatalf("[%s] [open] gate expected %v found %v", serviceType, v.expectedAfterClose, result)
		}
		// open gate
		store.GateOpen(sk)
		result = store.IsGateOpen(sk)
		if v.expectedAfterOpen != result {
			t.Fatalf("[%s] [close] gate expected %v found %v", serviceType, v.expectedAfterOpen, result)
		}
	}

}
