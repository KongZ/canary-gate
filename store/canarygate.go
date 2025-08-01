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
	"fmt"
	"os"

	piggysecv1alpha1 "github.com/KongZ/canary-gate/api/v1alpha1"
	"github.com/KongZ/canary-gate/controller"
	"github.com/KongZ/canary-gate/service"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	kubernetesConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// CanaryGateStatus defines the observed state of CanaryGate
type CanaryGateStatus struct {
	// Name of the canary
	Name string `json:"name"`
	// Namespace of the canary
	Namespace string `json:"namespace"`
	// Gate status
	Status string `json:"status"`
	// Gate Message
	Message string `json:"message,omitempty"`
}

type CanaryGateStore struct {
	k8sClient dynamic.Interface
	configNS  string
	event     record.EventBroadcaster
	recorder  record.EventRecorderLogger
}

var GroupVersionResource = schema.GroupVersionResource{
	Group:    piggysecv1alpha1.GroupVersion.Group,
	Version:  piggysecv1alpha1.GroupVersion.Version,
	Resource: "canarygates",
}

// CanaryGateStore creates new Kubernetes CRD to store gate states.
// CanaryGate CRD is created in the namespace specified by the environment variable CANARY_GATE_NAMESPACE.
// The CRD name is constructed as "<name>" in the namespace CANARY_GATE_NAMESPACE.
func NewCanaryGateStore(k8sClient dynamic.Interface) (Store, error) {
	var k8s dynamic.Interface
	var err error
	if k8sClient == nil {
		k8s, err = newDynamicClient()
		if err != nil {
			log.Error().Msgf("error creating k8s client: %s", err)
		}
	} else {
		k8s = k8sClient
	}
	// Setup EventBroadcaster to send events to the Kubernetes API server
	eventBroadcaster := record.NewBroadcaster()
	// Create an instance of our custom DynamicEventSink.
	dynamicSink := &controller.DynamicEventSink{
		Client: k8s,
	}
	scheme := runtime.NewScheme()
	if err = piggysecv1alpha1.AddToScheme(scheme); err != nil {
		log.Error().Msgf("error creating k8s scheme: %s", err)
	}
	// Tell the broadcaster to use our custom sink.
	eventBroadcaster.StartRecordingToSink(dynamicSink)
	store := &CanaryGateStore{
		k8sClient: k8s,
		configNS:  os.Getenv("CANARY_GATE_NAMESPACE"),
		event:     eventBroadcaster,
		recorder:  eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: "canarygate"}),
	}
	return store, nil
}

func newDynamicClient() (dynamic.Interface, error) {
	kubeConfig, err := kubernetesConfig.GetConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

// getCanaryGateNamespace get location of canarygate crd
// if key does not contains the target namespace, use the configured namespace
func (s *CanaryGateStore) getCanaryGateNamespace(key StoreKey) string {
	if s.configNS != "" {
		return s.configNS
	}
	return key.Namespace
}

// getPod get pod from namespace and name.
func (s *CanaryGateStore) targetName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// // getPod get pod from namespace and name.
// func (s *CanaryGateStore) getCanaryGate(namespace, name string) (piggysecv1alpha1.CanaryGate, error) {
// 	unstructuredPod, err := s.k8sClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
// 	if err != nil {
// 		return piggysecv1alpha1.CanaryGate{}, fmt.Errorf("could not find canarygates '%s' in namespace '%s' to attach an event to. Error: %v\n", name, namespace, err)
// 	}
// 	var gate piggysecv1alpha1.CanaryGate
// 	runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPod.Object, &gate)
// 	return gate, nil
// }

// createCanaryGate create Canary Gate object on target namespace.
func (s *CanaryGateStore) CreateCanaryGate(ctx context.Context, key StoreKey) *piggysecv1alpha1.CanaryGate {
	gateNs := s.getCanaryGateNamespace(key)
	canaryGate := &piggysecv1alpha1.CanaryGate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", GroupVersionResource.Group, GroupVersionResource.Version),
			Kind:       "CanaryGate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: gateNs,
		},
	}

	// Convert the typed object to an unstructured object
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(canaryGate)
	if err != nil {
		return canaryGate
	}

	// Create the resource using the dynamic client
	_, err = s.k8sClient.Resource(GroupVersionResource).Namespace(gateNs).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	if err != nil {
		log.Error().Msgf("Error while creating canarygate [%s/%s] %v. Gate [%s] is set to [%s]", gateNs, key.Name, err, key.String(), defaultText(key))
	}
	return canaryGate
}

func (s *CanaryGateStore) GetCanaryGate(ctx context.Context, key StoreKey) (*piggysecv1alpha1.CanaryGate, error) {
	gateNs := s.getCanaryGateNamespace(key)
	unstructuredObj, err := s.k8sClient.Resource(GroupVersionResource).Namespace(gateNs).Get(ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var gate piggysecv1alpha1.CanaryGate
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &gate)
	if err != nil {
		return nil, err
	}

	return &gate, nil
}

func (s *CanaryGateStore) CreateCanaryGateAndGet(ctx context.Context, key StoreKey) (*piggysecv1alpha1.CanaryGate, error) {
	gateNs := s.getCanaryGateNamespace(key)
	conf, err := s.GetCanaryGate(ctx, key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load configmap [%s/%s].", gateNs, key.Name)
			_ = s.CreateCanaryGate(ctx, key)
			return s.GetCanaryGate(ctx, key) // Reload to ensure we have the latest version
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load configmap [%s/%s] %v.", gateNs, key.Name, statusError.ErrStatus.Message)
			return nil, err
		}
	}
	return conf, nil
}

func (s *CanaryGateStore) UpdateCanaryGate(ctx context.Context, key StoreKey, val bool) {
	gateNs := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		conf, err := s.CreateCanaryGateAndGet(ctx, key)
		if err != nil {
			return err
		}
		// update gate fields
		status := GateStatus(val)
		switch key.Type {
		case service.HookConfirmRollout:
			conf.Spec.ConfirmRollout = status
		case service.HookPreRollout:
			conf.Spec.PreRollout = status
		case service.HookRollout:
			conf.Spec.Rollout = status
		case service.HookConfirmTrafficIncrease:
			conf.Spec.ConfirmTrafficIncrease = status
		case service.HookPostRollout:
			conf.Spec.PostRollout = status
		case service.HookConfirmPromotion:
			conf.Spec.ConfirmPromotion = status
		case service.HookRollback:
			conf.Spec.Rollback = status
		}
		conf.Status.Name = key.Name
		conf.Status.Namespace = key.Namespace
		conf.Status.Target = s.targetName(key.Namespace, key.Name)

		// Convert back to unstructured
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(conf)
		if err != nil {
			return err
		}
		log.Trace().Msgf("Saving to canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, conf.Name, key, status)
		_, err = s.k8sClient.Resource(GroupVersionResource).Namespace(gateNs).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Recording event [%s/%s]. Gate [%s] is set to [%s]", gateNs, conf.Name, key, status)
		s.UpdateEvent(ctx, key, "Updated", fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), status))
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update canarygate [%s/%s] %v.", gateNs, key.Name, retryErr)
	}
}

func (s *CanaryGateStore) GetLastEvent(ctx context.Context, key StoreKey) string {
	gate, err := s.GetCanaryGate(ctx, key)
	if err != nil {
		return ""
	}
	return gate.Status.Message
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (s *CanaryGateStore) UpdateEvent(ctx context.Context, key StoreKey, status string, message string) {
	gateNs := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		conf, err := s.CreateCanaryGateAndGet(ctx, key)
		if err != nil {
			return err
		}
		conf.Status.Name = key.Name
		conf.Status.Namespace = key.Namespace
		conf.Status.Status = status
		conf.Status.Message = message
		conf.Status.Target = s.targetName(key.Namespace, key.Name)
		// Convert back to unstructured
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(conf)
		if err != nil {
			return err
		}
		_, err = s.k8sClient.Resource(GroupVersionResource).Namespace(gateNs).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Updating canarygate [%s/%s] status", gateNs, conf.Name)
		if message != "" {
			if gate, err := s.GetCanaryGate(ctx, key); err == nil {
				s.recorder.Event(
					gate,                   // The object the event is about.
					corev1.EventTypeNormal, // The type of event.
					status,                 // A brief reason.
					message,                // A human-readable message.
				)
			}
		}
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update canarygate [%s/%s] %v.", gateNs, key.Name, retryErr)
	}
}

func (s *CanaryGateStore) GateOpen(key StoreKey) {
	s.UpdateCanaryGate(context.TODO(), key, true)
}

func (s *CanaryGateStore) GateClose(key StoreKey) {
	s.UpdateCanaryGate(context.TODO(), key, false)
}

func (s *CanaryGateStore) IsGateOpen(key StoreKey) bool {
	gateNs := s.getCanaryGateNamespace(key)
	conf, err := s.CreateCanaryGateAndGet(context.Background(), key)
	if err != nil {
		log.Warn().Msgf("Unable to load canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, key.Name, key, defaultText(key))
		return defaultValue(key)
	}
	status := ""
	if conf != nil {
		switch key.Type {
		case service.HookConfirmRollout:
			status = conf.Spec.ConfirmRollout
		case service.HookPreRollout:
			status = conf.Spec.PreRollout
		case service.HookRollout:
			status = conf.Spec.Rollout
		case service.HookConfirmTrafficIncrease:
			status = conf.Spec.ConfirmTrafficIncrease
		case service.HookPostRollout:
			status = conf.Spec.PostRollout
		case service.HookConfirmPromotion:
			status = conf.Spec.ConfirmPromotion
		case service.HookRollback:
			status = conf.Spec.Rollback
		}
	}
	log.Trace().Msgf("Loading from canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, key.Name, key, status)
	if status == "" {
		return defaultValue(key)
	}
	return GateBoolStatus(status)
}

func (s *CanaryGateStore) Shutdown() error {
	s.event.Shutdown()
	return nil
}
