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
	"time"

	// A popular assertion library
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestCanaryGate(t *testing.T) {
	for _, v := range typeCases {
		serviceType := v.serviceType
		sk := StoreKey{
			Namespace: "canary-ns",
			Name:      "test-canary",
			Type:      serviceType,
		}
		// Create a fake clientset
		scheme := runtime.NewScheme()
		f := fake.NewSimpleDynamicClient(scheme)

		// Tests
		store, err := NewCanaryGateStore(f)
		require.NoError(t, err, "createCanaryGate should not return an error")
		if err != nil {
			t.Error(err)
		}
		result := store.IsGateOpen(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to create
		require.Equalf(t, v.expectedInit, result, "[%s] [default] gate expected %v found %v", serviceType, v.expectedInit, result)

		// close gate
		store.GateClose(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to close
		result = store.IsGateOpen(sk)
		require.Equalf(t, v.expectedAfterClose, result, "[%s] is [closed] gate expected %v found %v", serviceType, v.expectedAfterClose, result)

		// open gate
		store.GateOpen(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to open
		result = store.IsGateOpen(sk)
		require.Equalf(t, v.expectedAfterOpen, result, "[%s] is [opened] gate expected %v found %v", serviceType, v.expectedAfterOpen, result)

		// shutdown store
		err = store.Shutdown()
		require.NoError(t, err, "Shutdown should not return an error")
	}
}

func TestCanaryGateEvent(t *testing.T) {
	sk := StoreKey{
		Namespace: "canary-ns",
		Name:      "test-canary",
	}
	// Create a fake clientset
	scheme := runtime.NewScheme()
	f := fake.NewSimpleDynamicClient(scheme)

	// Tests
	store, err := NewCanaryGateStore(f)
	require.NoError(t, err, "createCanaryGate should not return an error")
	if err != nil {
		t.Error(err)
	}
	// Check initial event state
	result := store.GetLastEvent(context.TODO(), sk)
	require.EqualValuesf(t, "", result, "Event should be empty, found %s", result)
	eventMessage := "Test event message"
	store.UpdateEvent(context.TODO(), sk, "status", eventMessage)
	result = store.GetLastEvent(context.TODO(), sk)
	require.EqualValuesf(t, eventMessage, result, "Event message should be '%s', found '%s'", eventMessage, result)
}
