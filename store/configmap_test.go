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
	"strconv"
	"testing"
	"time"

	"github.com/KongZ/canary-gate/service"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	// k8stesting "k8s.io/client-go/testing"
)

func StrToBool(str string) bool {
	if value, err := strconv.ParseBool(str); err == nil {
		return value
	}
	return false
}

type TestCase struct {
	serviceType        service.HookType
	expectedInit       bool
	expectedAfterClose bool
	expectedAfterOpen  bool
}

var typeCases = []TestCase{
	{
		service.HookConfirmRollout, true, false, true,
	},
	{
		service.HookRollout, true, false, true,
	},
	{
		service.HookConfirmPromotion, true, false, true,
	},
	{
		service.HookConfirmTrafficIncrease, true, false, true,
	},
	{
		service.HookPostRollout, true, false, true,
	},
	{
		service.HookPreRollout, true, false, true,
	},
	{
		service.HookRollback, false, false, true,
	},
}

func TestConfigMapGate(t *testing.T) {
	for _, v := range typeCases {
		serviceType := v.serviceType
		sk := StoreKey{
			Namespace: "canary-ns",
			Name:      "test-canary",
			Type:      serviceType,
		}
		// Create a fake clientset
		f := fake.NewSimpleClientset()
		// f.PrependReactor("create", "configmap", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		// 	configMap := &corev1.ConfigMap{
		// 		ObjectMeta: metav1.ObjectMeta{Name: "test-canary"},
		// 		Data:       map[string]string{},
		// 	}
		// 	configMap.Data["rollout"] = GATE_OPEN
		// 	return false, nil, nil
		// })

		// Tests
		store, err := NewConfigMapStore(f)
		if err != nil {
			t.Error(err)
		}
		result := store.IsGateOpen(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to create
		if v.expectedInit != result {
			t.Fatalf("[%s] [default] gate expected %v found %v", serviceType, v.expectedInit, result)
		}
		// close gate
		store.GateClose(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to close
		result = store.IsGateOpen(sk)
		if v.expectedAfterClose != result {
			t.Fatalf("[%s] is [closed] gate expected %v found %v", serviceType, v.expectedAfterClose, result)
		}
		// open gate
		store.GateOpen(sk)
		time.Sleep(10 * time.Millisecond) // wait for gate to open
		result = store.IsGateOpen(sk)
		if v.expectedAfterOpen != result {
			t.Fatalf("[%s] is [opened] gate expected %v found %v", serviceType, v.expectedAfterOpen, result)
		}
		// shutdown store
		err = store.Shutdown()
		require.NoError(t, err, "Shutdown should not return an error")
	}
}

func TestConfigMapEvent(t *testing.T) {
	sk := StoreKey{
		Namespace: "canary-ns",
		Name:      "test-canary",
	}
	f := fake.NewSimpleClientset()
	store, err := NewConfigMapStore(f)
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
