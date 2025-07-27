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
	"testing"

	"github.com/stretchr/testify/require"
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
		// shutdown store
		err = store.Shutdown()
		require.NoError(t, err, "Shutdown should not return an error")
	}
}

func TestMemoryGateEvent(t *testing.T) {
	sk := StoreKey{
		Namespace: "canary-ns",
		Name:      "test-canary",
	}
	store, err := NewMemoryStore()
	if err != nil {
		t.Error(err)
	}
	result := store.GetLastEvent(context.TODO(), sk)
	require.EqualValuesf(t, "", result, "Event should be empty, found %s", result)
	eventMessage := "Test event message"
	store.UpdateEvent(context.TODO(), sk, "status", eventMessage)
	result = store.GetLastEvent(context.TODO(), sk)
	require.EqualValuesf(t, eventMessage, result, "Event message should be '%s', found '%s'", eventMessage, result)
}
