package store

import (
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
	}
}
