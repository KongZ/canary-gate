package store

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	kubernetesConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const ConfigMapSuffix = "cgate"

type ConfigMapStore struct {
	data      *sync.Map
	k8sClient kubernetes.Interface
	configNS  string
}

// NewConfigMapStore creates a new ConfigMapStore instance.
// ConfigMapstore uses Kubernetes ConfigMaps to store gate states.
// ConfirMaps are created in the namespace specified by the environment variable CANARY_GATE_NAMESPACE.
// The ConfigMap name is constructed as "<namespace>-<name>-cgate".
func NewConfigMapStore(k8sClient kubernetes.Interface) (Store, error) {
	var k8s kubernetes.Interface
	var err error
	if k8sClient == nil {
		k8s, err = newK8sClient()
		if err != nil {
			log.Error().Msgf("error creating k8s client: %s", err)
		}
	} else {
		k8s = k8sClient
	}
	store := &ConfigMapStore{
		data:      new(sync.Map),
		k8sClient: k8s,
		configNS:  os.Getenv("CANARY_GATE_NAMESPACE"),
	}
	return store, nil
}

func newK8sClient() (kubernetes.Interface, error) {
	kubeConfig, err := kubernetesConfig.GetConfig()
	if err != nil {
		return nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	return k8sClient, nil
}

// getConfigMapName get store key name
func (s *ConfigMapStore) getConfigMapName(key StoreKey) string {
	return fmt.Sprintf("%s-%s-%s", key.Namespace, key.Name, ConfigMapSuffix)
}

// getConfigMapNamespace get location of configmap
func (s *ConfigMapStore) getConfigMapNamespace(key StoreKey) string {
	if s.configNS != "" {
		return s.configNS
	}
	return key.Namespace
}

// StoreKey get store key name
func (s *ConfigMapStore) createConfigMap(key StoreKey) *corev1.ConfigMap {
	confName := s.getConfigMapName(key)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: confName},
		Data:       map[string]string{},
	}
	ns := s.getConfigMapNamespace(key)
	configMap.Data[string(key.Type)] = GateStatus(defaultValue(key))
	_, err := s.k8sClient.CoreV1().ConfigMaps(ns).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		log.Error().Msgf("Error while creating configmap [%s/%s] %v. Gate [%s] is set to [%s]", ns, confName, err, key.String(), defaultText(key))
	}
	return configMap
}

func (s *ConfigMapStore) updateGate(key StoreKey, val bool) {
	confName := s.getConfigMapName(key)
	ns := s.getConfigMapNamespace(key)
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// defer cancel()
		conf, err := s.k8sClient.CoreV1().ConfigMaps(ns).Get(context.TODO(), confName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load configmap [%s/%s].", ns, confName)
				conf = s.createConfigMap(key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load configmap [%s/%s] %v.", ns, confName, statusError.ErrStatus.Message)
				return err
			}
		}
		conf.Data[string(key.Type)] = GateStatus(val)
		_, err = s.k8sClient.CoreV1().ConfigMaps(ns).Update(context.TODO(), conf, metav1.UpdateOptions{})
		log.Trace().Msgf("Saving to configmap [%s/%s]. Gate [%s] is set to [%s]", ns, conf.Name, key, conf.Data[string(key.Type)])
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update configmap [%s/%s] %v.", ns, confName, retryErr)
		// cancel()
	}
}

func (s *ConfigMapStore) GateOpen(key StoreKey) {
	s.updateGate(key, true)
}

func (s *ConfigMapStore) GateClose(key StoreKey) {
	s.updateGate(key, false)
}

func (s *ConfigMapStore) IsGateOpen(key StoreKey) bool {
	confName := s.getConfigMapName(key)
	ns := s.getConfigMapNamespace(key)
	conf, err := s.k8sClient.CoreV1().ConfigMaps(ns).Get(context.TODO(), confName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load configmap [%s/%s]. Gate [%s] is set to [%s]", ns, confName, key, defaultText(key))
			conf = s.createConfigMap(key)
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load configmap [%s/%s] %v. Gate [%s] is set to [%s]", ns, confName, statusError.ErrStatus.Message, key.String(), defaultText(key))
			return defaultValue(key)
		}
	}
	val, ok := conf.Data[string(key.Type)]
	log.Trace().Msgf("Loading from configmap [%s/%s]. Gate [%s] is set to [%s]", ns, conf.Name, key, val)
	if ok {
		return val == GATE_OPEN
	}
	return defaultValue(key)
}

func (s *ConfigMapStore) Shutdown() error {
	return nil
}
