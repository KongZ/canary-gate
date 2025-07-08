package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/KongZ/canary-gate/service"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	kubernetesConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const ApiGroup = "piggysec.com"
const ApiVersion = "v1alpha1"
const FlaggerApiVersion = "v1beta1"

// CanaryGateSpec defines the desired state of CanaryGate
type CanaryGateSpec struct {
	ConfirmRollout         string `json:"confirm-rollout,omitempty"`
	PreRollout             string `json:"pre-rollout,omitempty"`
	Rollout                string `json:"rollout,omitempty"`
	ConfirmTrafficIncrease string `json:"confirm-traffic-increase,omitempty"`
	ConfirmPromotion       string `json:"confirm-promotion,omitempty"`
	PostRollout            string `json:"post-rollout,omitempty"`
	Rollback               string `json:"rollback,omitempty"`

	// Flagger holds arbitrary configuration data.
	// +kubebuilder:pruning:PreserveUnknownFields
	// Flagger runtime.RawExtension `json:"flagger,omitempty"`
	Flagger *FlaggerSpec `json:"flagger,omitempty"`
}

// This is the new Webhook struct
type Webhook struct {
	Name     string               `json:"name,omitempty"`
	URL      string               `json:"url,omitempty"`
	Timeout  string               `json:"timeout,omitempty"`
	Metadata runtime.RawExtension `json:"metadata,omitempty"`
}

// This is the new AnalysisSpec struct
type AnalysisSpec struct {
	Webhooks []Webhook `json:"webhooks,omitempty"`
}

// This is the new FlaggerSpec struct. It only defines the 'analysis' field,
// but because the CRD has 'x-kubernetes-preserve-unknown-fields: true',
// other fields will be preserved during a read-modify-write cycle.
type FlaggerSpec struct {
	Analysis *AnalysisSpec `json:"analysis,omitempty"`
}

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

// +k8s:deepcopy-gen:interfaces=k8s.io.apimachinery.pkg/runtime.Object
// CanaryGate is the Schema for the canarygates API
type CanaryGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanaryGateSpec   `json:"spec,omitempty"`
	Status CanaryGateStatus `json:"status,omitempty"`
}

// FlaggerCanarySpec defines the desired state of Flagger Cnaary
type FlaggerCanarySpec struct {
	// Spec holds arbitrary configuration data.
	// +kubebuilder:pruning:PreserveUnknownFields
	Raw runtime.RawExtension `json:"flagger,omitempty"`
}

// FlaggerCanary is the Schema for the flagger canary API
type FlaggerCanary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FlaggerCanarySpec `json:"spec,omitempty"`
	// This now points to our new struct, providing structured access
	Flagger *FlaggerSpec `json:"flagger,omitempty"`
}

type CanaryGateStore struct {
	k8sClient dynamic.Interface
	// canaryGate CanaryGate
	// key        StoreKey
	configNS string
}

var gvr = schema.GroupVersionResource{
	Group:    ApiGroup,
	Version:  ApiVersion,
	Resource: "canarygates",
}

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
	store := &CanaryGateStore{
		k8sClient: k8s,
		configNS:  os.Getenv("CANARY_GATE_NAMESPACE"),
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

// getCanaryGateName get store key name
func (s *CanaryGateStore) getCanaryGateName(key StoreKey) string {
	return fmt.Sprintf("%s-%s", key.Namespace, key.Name)
}

// getCanaryGateNamespace get location of configmap
func (s *CanaryGateStore) getCanaryGateNamespace(key StoreKey) string {
	if s.configNS != "" {
		return s.configNS
	}
	return key.Namespace
}

// updateWebhookURL demonstrates how to modify a field within the newly structured part of the CRD.
func (s *CanaryGateStore) UpdateWebhookURL(ctx context.Context, key StoreKey, webhookName, newURL string) (*CanaryGate, error) {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	fmt.Printf("--- Attempting to update webhook '%s' for '%s' ---\n", webhookName, confName)

	// 1. READ: Get the latest version of the object
	gate, err := s.GetCanaryGate(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get canary gate: %w", err)
	}

	// 2. MODIFY: Access the structured field path safely
	found := false
	// Nil checks are important for safety!
	if gate.Spec.Flagger != nil && gate.Spec.Flagger.Analysis != nil {
		for i, wh := range gate.Spec.Flagger.Analysis.Webhooks {
			if wh.Name == webhookName {
				fmt.Printf("Found webhook. Old URL: '%s', New URL: '%s'\n", gate.Spec.Flagger.Analysis.Webhooks[i].URL, newURL)
				// Update the URL directly on the struct
				gate.Spec.Flagger.Analysis.Webhooks[i].URL = newURL
				found = true
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("webhook with name '%s' not found in analysis spec", webhookName)
	}

	// Convert the entire modified 'gate' object back to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(gate)
	if err != nil {
		return nil, fmt.Errorf("failed to convert gate to unstructured: %w", err)
	}

	// 3. WRITE: Perform the update
	updatedUnstructured, err := s.k8sClient.Resource(gvr).Namespace(ns).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update canary gate: %w", err)
	}

	fmt.Println("Successfully updated webhook URL.")

	// Convert the result back to our typed struct to return it
	var updatedGate CanaryGate
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(updatedUnstructured.Object, &updatedGate); err != nil {
		return nil, fmt.Errorf("failed to convert updated unstructured object: %w", err)
	}

	return &updatedGate, nil
}

// createFlaggerCanary create Flagger Canary object on target namespace.
func (s *CanaryGateStore) CreateFlaggerCanary(ctx context.Context, key StoreKey, canaryGate CanaryGate) error {
	// Marshal it to raw JSON bytes
	flaggerRaw, err := json.Marshal(canaryGate.Spec.Flagger)
	if err != nil {
		return err
	}
	// Create our typed CanaryGate object
	flaggerCanary := &FlaggerCanary{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "flagger.app/" + FlaggerApiVersion,
			Kind:       "Canary",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		// TODO add webhooks
		Spec: FlaggerCanarySpec{
			Raw: runtime.RawExtension{Raw: flaggerRaw},
		},
	}

	// Convert the typed object to an unstructured object
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(flaggerCanary)
	if err != nil {
		return err
	}

	// Create the resource using the dynamic client
	// Flagger config is located at application namespace
	_, err = s.k8sClient.Resource(gvr).Namespace(key.Namespace).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	return err
}

// createCanaryGate create Canary Gate object on target namespace.
func (s *CanaryGateStore) CreateCanaryGate(ctx context.Context, key StoreKey) *CanaryGate {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	canaryGate := &CanaryGate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", ApiGroup, ApiVersion),
			Kind:       "CanaryGate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      confName,
			Namespace: ns,
		},
	}

	// // Define the flagger data as a simple map
	// flaggerConfig := map[string]interface{}{}
	// // Marshal it to raw JSON bytes
	// flaggerRaw, err := json.Marshal(flaggerConfig)
	// if err != nil {
	// 	return canaryGate
	// }
	// // Set fields on the passed-in CanaryGate object
	// canaryGate.Spec.Flagger = runtime.RawExtension{Raw: flaggerRaw}

	// Convert the typed object to an unstructured object
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(canaryGate)
	if err != nil {
		return canaryGate
	}

	// Create the resource using the dynamic client
	_, err = s.k8sClient.Resource(gvr).Namespace(ns).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	if err != nil {
		log.Error().Msgf("Error while creating canarygate [%s/%s] %v. Gate [%s] is set to [%s]", ns, confName, err, key.String(), defaultText(key))
	}
	return canaryGate
}

func (s *CanaryGateStore) GetCanaryGate(ctx context.Context, key StoreKey) (*CanaryGate, error) {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	unstructuredObj, err := s.k8sClient.Resource(gvr).Namespace(ns).Get(ctx, confName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var gate CanaryGate
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &gate)
	if err != nil {
		return nil, err
	}

	return &gate, nil
}

func (s *CanaryGateStore) CreateCanaryGateAndGet(ctx context.Context, key StoreKey) *CanaryGate {
	_ = s.CreateCanaryGate(ctx, key)
	conf, _ := s.GetCanaryGate(ctx, key)
	return conf
}

func (s *CanaryGateStore) UpdateCanaryGate(ctx context.Context, key StoreKey, val bool) {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// First, get the latest version of the object
		conf, err := s.GetCanaryGate(ctx, key)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load configmap [%s/%s].", ns, confName)
				conf = s.CreateCanaryGateAndGet(ctx, key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load configmap [%s/%s] %v.", ns, confName, statusError.ErrStatus.Message)
				return err
			}
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
		// Convert back to unstructured
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(conf)
		if err != nil {
			return err
		}
		// TODO update gate status
		_, err = s.k8sClient.Resource(gvr).Namespace(ns).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Saving to canarygate [%s/%s]. Gate [%s] is set to [%s]", ns, conf.Name, key, status)
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update canarygate [%s/%s] %v.", ns, confName, retryErr)
	}
}

func (s *CanaryGateStore) UpdateCanaryGateStatus(ctx context.Context, key StoreKey, status string, message string) {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// First, get the latest version of the object
		conf, err := s.GetCanaryGate(ctx, key)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load configmap [%s/%s].", ns, confName)
				conf = s.CreateCanaryGateAndGet(ctx, key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load configmap [%s/%s] %v.", ns, confName, statusError.ErrStatus.Message)
				return err
			}
		}
		conf.Status.Name = key.Name
		conf.Status.Namespace = key.Namespace
		conf.Status.Status = status
		conf.Status.Message = message
		// Convert back to unstructured
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(conf)
		if err != nil {
			return err
		}
		// TODO update gate status
		_, err = s.k8sClient.Resource(gvr).Namespace(ns).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Updating canarygate [%s/%s] status", ns, conf.Name)
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update canarygate [%s/%s] %v.", ns, confName, retryErr)
	}
}

func (s *CanaryGateStore) GateOpen(key StoreKey) {
	s.UpdateCanaryGate(context.TODO(), key, true)
}

func (s *CanaryGateStore) GateClose(key StoreKey) {
	s.UpdateCanaryGate(context.TODO(), key, false)
}

func (s *CanaryGateStore) IsGateOpen(key StoreKey) bool {
	confName := s.getCanaryGateName(key)
	ns := s.getCanaryGateNamespace(key)
	conf, err := s.GetCanaryGate(context.TODO(), key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load canarygate [%s/%s]. Gate [%s] is set to [%s]", ns, confName, key, defaultText(key))
			// TODO create canarygate
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load canarygate [%s/%s] %v. Gate [%s] is set to [%s]", ns, confName, statusError.ErrStatus.Message, key.String(), defaultText(key))
			return defaultValue(key)
		}
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
	log.Trace().Msgf("Loading from canarygate [%s/%s]. Gate [%s] is set to [%s]", ns, confName, key, status)
	if status == "" {
		return defaultValue(key)
	}
	return GateBoolStatus(status)
}
