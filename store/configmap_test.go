package store

import (
	"strconv"
	"testing"
	"time"

	"github.com/KongZ/canary-gate/service"
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
	}

}
