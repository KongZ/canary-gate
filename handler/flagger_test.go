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
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	piggysecv1alpha1 "github.com/KongZ/canary-gate/api/v1alpha1"
	"github.com/KongZ/canary-gate/noti"
	"github.com/KongZ/canary-gate/service"
	"github.com/KongZ/canary-gate/store"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"k8s.io/apimachinery/pkg/runtime"
	dfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	confirmRolloutPath         = "/confirm-rollout"
	preRolloutPath             = "/pre-rollout"
	rolloutPath                = "/rollout"
	confirmTrafficIncreasePath = "/confirm-traffic-increase"
	confirmPromotionPath       = "/confirm-promotion"
	postRolloutPath            = "/post-rollout"
	rollbackPath               = "/rollback"
	eventPath                  = "/event"
)

func buildPayload[I any](payload *I) []byte {
	r, err := json.Marshal(&payload)
	if err == nil {
		return r
	}
	return []byte{}
}

func buildGatePayload(t service.HookType) []byte {
	payload := &CanaryGatePayload{
		Type:      t,
		Name:      "test-canary",
		Namespace: "canary-ns",
	}
	return buildPayload(payload)
}

func buildResponsePayloadStatus(t *testing.T, key store.StoreKey, status string, payload map[string][]CanaryGateStatus) []byte {
	for k := range payload {
		payload[k] = append(payload[k], CanaryGateStatus{
			Type:      service.HookEvent,
			Namespace: key.Namespace,
			Name:      key.Name,
			Status:    fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), status),
		})
	}
	r, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("Error while marshal payload %v", err)
	}
	return r
}

func buildResponsePayload(t *testing.T, payload map[string][]CanaryGateStatus) []byte {
	r, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("Error while marshal payload %v", err)
	}
	return r
}

func compareResult(t *testing.T, path string, expectedStatus, actualStatus int, expectedBody, actualBody []byte, wait bool) {
	require.EqualValues(t, expectedStatus, actualStatus, "Expected status code [%s] %v, got %v", path, expectedStatus, actualStatus)
	if expectedBody != nil {
		require.EqualValues(t, expectedBody, actualBody, "Expected body [%s] %s, got %s", path, string(expectedBody), string(actualBody))
	}
	if wait {
		time.Sleep(10 * time.Millisecond) // wait for k8s to update configmap
	}
}

func httpTest(t *testing.T, handlerFunc http.Handler, path string, payload []byte, expectedStatus int, expectedBody []byte) {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handlerFunc.ServeHTTP(w, req)
	resp := w.Result() // Get the http.Response
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Failed to close response body: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	// fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), GateStatus(val))
	compareResult(t, path, expectedStatus, w.Code, expectedBody, body, path == "/open" || path == "close")
}

func createTestCase(gateName service.HookType, payload *CanaryWebhookPayload) (flaggerPayload []byte, gatePayload []byte, gateOpenResponse map[string][]CanaryGateStatus, gateCloseResponse map[string][]CanaryGateStatus) {
	flaggerPayload = buildPayload(payload)
	gatePayload = buildGatePayload(gateName)
	gateOpenResponse = make(map[string][]CanaryGateStatus)
	gateCloseResponse = make(map[string][]CanaryGateStatus)
	mapKey := fmt.Sprintf("%s/%s", payload.Namespace, payload.Name)
	gateOpenResponse[mapKey] = []CanaryGateStatus{
		{Type: gateName, Namespace: payload.Namespace, Name: payload.Name, Status: string(store.GATE_OPEN)},
	}
	gateCloseResponse[mapKey] = []CanaryGateStatus{
		{Type: gateName, Namespace: payload.Namespace, Name: payload.Name, Status: string(store.GATE_CLOSE)},
	}
	return flaggerPayload, gatePayload, gateOpenResponse, gateCloseResponse
}

func testGate(t *testing.T, gateName string, storeName string) {
	cmd := &cli.Command{}
	// Create a fake clientset
	var storage store.Store
	var err error
	switch storeName {
	case "configmap":
		f := fake.NewSimpleClientset()
		storage, err = store.NewConfigMapStore(f)
		if err != nil {
			t.Error(err)
		}
	case "memory":
		storage, err = store.NewMemoryStore()
		if err != nil {
			t.Error(err)
		}
	default:
		scheme := runtime.NewScheme()
		if err := piggysecv1alpha1.AddToScheme(scheme); err != nil {
			log.Error().Msgf("error creating k8s scheme: %s", err)
		}
		f := dfake.NewSimpleDynamicClient(scheme)
		storage, err = store.NewCanaryGateStore(f)
		if err != nil {
			t.Error(err)
		}
	}
	handler := NewHandler(cmd, noti.NewQuietNoti(), storage)
	sKey := store.StoreKey{
		Namespace: "canary-ns",
		Name:      "test-canary",
	}
	// ConfigMap Store both target namespace and name are same as ConfigMap name
	// CanarayGateStore has target fields which are different from ConfigMapStore
	webhookPayload := &CanaryWebhookPayload{
		Name:      sKey.Name,
		Namespace: sKey.Namespace,
		Phase:     service.PhasePromoting,
		Checksum:  "",
		Metadata:  map[string]string{},
	}
	var handlerFunc http.Handler
	var flaggerPayload []byte
	var gatePayload []byte
	var gateOpenResponse map[string][]CanaryGateStatus
	var gateCloseResponse map[string][]CanaryGateStatus
	expectedStatus := []int{http.StatusOK, http.StatusForbidden, http.StatusOK}
	switch gateName {
	case confirmRolloutPath:
		handlerFunc = handler.ConfirmRollout()
		sKey.Type = service.HookConfirmRollout
	case preRolloutPath:
		handlerFunc = handler.PreRollout()
		sKey.Type = service.HookPreRollout
	case rolloutPath:
		handlerFunc = handler.Rollout()
		sKey.Type = service.HookRollout
	case confirmTrafficIncreasePath:
		handlerFunc = handler.ConfirmTrafficIncrease()
		sKey.Type = service.HookConfirmTrafficIncrease
	case confirmPromotionPath:
		handlerFunc = handler.ConfirmPromotion()
		sKey.Type = service.HookConfirmPromotion
	case postRolloutPath:
		handlerFunc = handler.PostRollout()
		sKey.Type = service.HookPostRollout
	case rollbackPath:
		handlerFunc = handler.Rollback()
		sKey.Type = service.HookRollback
		flaggerPayload, gatePayload, gateOpenResponse, gateCloseResponse = createTestCase(sKey.Type, webhookPayload)
		// rollback default is close
		httpTest(t, handlerFunc, gateName, flaggerPayload, http.StatusForbidden, nil)
		httpTest(t, handler.OpenGate(), "/open", gatePayload, http.StatusOK, buildResponsePayload(t, gateOpenResponse))
		httpTest(t, handlerFunc, gateName, flaggerPayload, http.StatusOK, nil)
		httpTest(t, handler.CloseGate(), "/close", gatePayload, http.StatusOK, buildResponsePayload(t, gateCloseResponse))
		httpTest(t, handlerFunc, gateName, flaggerPayload, http.StatusForbidden, nil)
		return
	case eventPath:
		handlerFunc = handler.Event()
		flaggerPayload, _, _, _ = createTestCase(sKey.Type, webhookPayload)
		httpTest(t, handlerFunc, gateName, flaggerPayload, http.StatusOK, nil)
		return
	}
	flaggerPayload, gatePayload, gateOpenResponse, gateCloseResponse = createTestCase(sKey.Type, webhookPayload)
	httpTest(t, handlerFunc, gateName, flaggerPayload, expectedStatus[0], nil)
	httpTest(t, handler.CloseGate(), "/close", gatePayload, http.StatusOK, buildResponsePayload(t, gateCloseResponse))
	httpTest(t, handler.StatusGate(), "/status", gatePayload, http.StatusOK, buildResponsePayloadStatus(t, sKey, store.GATE_CLOSE, gateCloseResponse))
	httpTest(t, handlerFunc, gateName, flaggerPayload, expectedStatus[1], nil)
	httpTest(t, handler.OpenGate(), "/open", gatePayload, http.StatusOK, buildResponsePayload(t, gateOpenResponse))
	httpTest(t, handler.StatusGate(), "/status", gatePayload, http.StatusOK, buildResponsePayloadStatus(t, sKey, store.GATE_OPEN, gateOpenResponse))
	httpTest(t, handlerFunc, gateName, flaggerPayload, expectedStatus[2], nil)
}

func TestConfirmRolloutHandler(t *testing.T) {
	testGate(t, confirmRolloutPath, "memory")
	testGate(t, confirmRolloutPath, "configmap")
	testGate(t, confirmRolloutPath, "canarygate")
}

func TestPreRolloutHandler(t *testing.T) {
	testGate(t, preRolloutPath, "memory")
	testGate(t, preRolloutPath, "configmap")
	testGate(t, preRolloutPath, "canarygate")
}

func TestRolloutHandler(t *testing.T) {
	testGate(t, rolloutPath, "memory")
	testGate(t, rolloutPath, "configmap")
	testGate(t, rolloutPath, "canarygate")
}

func TestConfirmTrafficIncreaseHandler(t *testing.T) {
	testGate(t, confirmTrafficIncreasePath, "memory")
	testGate(t, confirmTrafficIncreasePath, "configmap")
	testGate(t, confirmTrafficIncreasePath, "canarygate")
}

func TestRollbackHandler(t *testing.T) {
	testGate(t, rollbackPath, "memory")
	testGate(t, rollbackPath, "configmap")
	testGate(t, rollbackPath, "canarygate")
}

func TestConfirmPromotionHandler(t *testing.T) {
	testGate(t, confirmPromotionPath, "memory")
	testGate(t, confirmPromotionPath, "configmap")
	testGate(t, confirmPromotionPath, "canarygate")
}

func TestPostRolloutHandler(t *testing.T) {
	testGate(t, postRolloutPath, "memory")
	testGate(t, postRolloutPath, "configmap")
	testGate(t, postRolloutPath, "canarygate")
}

func TestPostEventHandler(t *testing.T) {
	testGate(t, eventPath, "memory")
	testGate(t, eventPath, "configmap")
	testGate(t, eventPath, "canarygate")
}

// mux.Handle("/event", handler.Event())
// mux.Handle("/open", handler.OpenGate())
// mux.Handle("/close", handler.CloseGate())
// mux.Handle("/status", handler.StatusGate())
// mux.Handle("/metrics", promhttp.Handler())
// mux.Handle("/version", serverHandler.Version())
