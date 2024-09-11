package store

import (
	"strconv"
	"testing"

	"github.com/KongZ/canary-gate/service"
	"k8s.io/client-go/kubernetes/fake"
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
		// Prepend a reactor to intercept Pod creation requests and store them
		// Create a fake clientset
		f := fake.NewSimpleClientset()
		// f.PrependReactor("create", "configmap", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		// 	configMap := &corev1.ConfigMap{
		// 		ObjectMeta: metav1.ObjectMeta{Name: "test-canary"},
		// 		Data:       map[string]string{},
		// 	}
		// 	configMap.Data["rollout"] = strconv.FormatBool(expectedRollOut)
		// 	return false, nil, nil
		// })

		// Tests
		store, err := NewConfigMapStore(f)
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
