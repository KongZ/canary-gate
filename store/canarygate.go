package store

import (
	"context"
	"fmt"
	"os"

	piggysecv1alpha1 "github.com/KongZ/canary-gate/api/v1alpha1"
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
}

var gvr = schema.GroupVersionResource{
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

// getCanaryGateNamespace get location of canarygate crd
// if key does not contains the target namespace, use the configured namespace
func (s *CanaryGateStore) getCanaryGateNamespace(key StoreKey) string {
	if s.configNS != "" {
		return s.configNS
	}
	return key.Namespace
}

// createCanaryGate create Canary Gate object on target namespace.
func (s *CanaryGateStore) CreateCanaryGate(ctx context.Context, key StoreKey) *piggysecv1alpha1.CanaryGate {
	gateNs := s.getCanaryGateNamespace(key)
	canaryGate := &piggysecv1alpha1.CanaryGate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", gvr.Group, gvr.Version),
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
	_, err = s.k8sClient.Resource(gvr).Namespace(gateNs).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	if err != nil {
		log.Error().Msgf("Error while creating canarygate [%s/%s] %v. Gate [%s] is set to [%s]", gateNs, key.Name, err, key.String(), defaultText(key))
	}
	return canaryGate
}

func (s *CanaryGateStore) GetCanaryGate(ctx context.Context, key StoreKey) (*piggysecv1alpha1.CanaryGate, error) {
	gateNs := s.getCanaryGateNamespace(key)
	unstructuredObj, err := s.k8sClient.Resource(gvr).Namespace(gateNs).Get(ctx, key.Name, metav1.GetOptions{})
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

func (s *CanaryGateStore) CreateCanaryGateAndGet(ctx context.Context, key StoreKey) *piggysecv1alpha1.CanaryGate {
	_ = s.CreateCanaryGate(ctx, key)
	conf, _ := s.GetCanaryGate(ctx, key)
	return conf
}

func (s *CanaryGateStore) UpdateCanaryGate(ctx context.Context, key StoreKey, val bool) {
	gateNs := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// First, get the latest version of the object
		conf, err := s.GetCanaryGate(ctx, key)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load canarygate [%s/%s].", gateNs, key.Name)
				conf = s.CreateCanaryGateAndGet(ctx, key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load canarygate [%s/%s] %v.", gateNs, key.Name, statusError.ErrStatus.Message)
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
		_, err = s.k8sClient.Resource(gvr).Namespace(gateNs).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Saving to canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, conf.Name, key, status)
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update canarygate [%s/%s] %v.", gateNs, key.Name, retryErr)
	}
}

func (s *CanaryGateStore) UpdateCanaryGateStatus(ctx context.Context, key StoreKey, status string, message string) {
	gateNs := s.getCanaryGateNamespace(key)
	// Perform the update
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// First, get the latest version of the object
		conf, err := s.GetCanaryGate(ctx, key)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load configmap [%s/%s].", gateNs, key.Name)
				conf = s.CreateCanaryGateAndGet(ctx, key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load configmap [%s/%s] %v.", gateNs, key.Name, statusError.ErrStatus.Message)
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
		_, err = s.k8sClient.Resource(gvr).Namespace(gateNs).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
		log.Trace().Msgf("Updating canarygate [%s/%s] status", gateNs, conf.Name)
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
	conf, err := s.GetCanaryGate(context.TODO(), key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, key.Name, key, defaultText(key))
			// TODO create canarygate
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load canarygate [%s/%s] %v. Gate [%s] is set to [%s]", gateNs, key.Name, statusError.ErrStatus.Message, key.String(), defaultText(key))
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
	log.Trace().Msgf("Loading from canarygate [%s/%s]. Gate [%s] is set to [%s]", gateNs, key.Name, key, status)
	if status == "" {
		return defaultValue(key)
	}
	return GateBoolStatus(status)
}
