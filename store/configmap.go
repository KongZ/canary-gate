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
	"sync"

	"github.com/KongZ/canary-gate/service"
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
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ctx := context.Background()
		conf, err := s.CreateConfigMapAndGet(ctx, key)
		if err != nil {
			return err
		}
		conf.Data[string(key.Type)] = GateStatus(val)
		log.Trace().Msgf("Saving to configmap [%s/%s]. Gate [%s] is set to [%s]", conf.Namespace, conf.Name, key, conf.Data[string(key.Type)])
		_, err = s.k8sClient.CoreV1().ConfigMaps(conf.Namespace).Update(ctx, conf, metav1.UpdateOptions{})
		log.Trace().Msgf("Recording event [%s/%s]. Gate [%s] is set to [%s]", conf.Namespace, conf.Name, key, GateStatus(val))
		s.UpdateEvent(ctx, key, "Updated", fmt.Sprintf("Gate [%s] is set to [%s]", key.String(), GateStatus(val)))
		return err
	})
	if retryErr != nil {
		confName := s.getConfigMapName(key)
		ns := s.getConfigMapNamespace(key)
		log.Error().Msgf("Unable to update configmap [%s/%s] %v.", ns, confName, retryErr)
	}
}

func (s *ConfigMapStore) GateOpen(key StoreKey) {
	s.updateGate(key, true)
}

func (s *ConfigMapStore) GateClose(key StoreKey) {
	s.updateGate(key, false)
}

func (s *ConfigMapStore) IsGateOpen(key StoreKey) bool {
	conf, err := s.CreateConfigMapAndGet(context.Background(), key)
	if err != nil {
		return defaultValue(key)
	}
	val, ok := conf.Data[string(key.Type)]
	log.Trace().Msgf("Loading from configmap [%s/%s]. Gate [%s] is set to [%s]", conf.Namespace, conf.Name, key, val)
	if ok {
		return val == GATE_OPEN
	}
	return defaultValue(key)
}

func (s *ConfigMapStore) Shutdown() error {
	return nil
}

func (s *ConfigMapStore) GetConfigMap(ctx context.Context, key StoreKey) (*corev1.ConfigMap, error) {
	confName := s.getConfigMapName(key)
	ns := s.getConfigMapNamespace(key)
	return s.k8sClient.CoreV1().ConfigMaps(ns).Get(ctx, confName, metav1.GetOptions{})
}

func (s *ConfigMapStore) CreateConfigMapAndGet(ctx context.Context, key StoreKey) (*corev1.ConfigMap, error) {
	confName := s.getConfigMapName(key)
	ns := s.getConfigMapNamespace(key)
	conf, err := s.GetConfigMap(ctx, key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load configmap [%s/%s].", ns, confName)
			_ = s.createConfigMap(key)
			return s.GetConfigMap(ctx, key) // Reload to ensure we have the latest version
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load configmap [%s/%s] %v.", ns, confName, statusError.ErrStatus.Message)
			return nil, err
		}
	}
	return conf, err
}

func (s *ConfigMapStore) UpdateEvent(ctx context.Context, key StoreKey, status string, message string) {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		conf, err := s.CreateConfigMapAndGet(ctx, key)
		if err != nil {
			return err
		}
		conf.Data[string(service.HookEvent)] = message
		log.Trace().Msgf("Saving to configmap [%s/%s]. Status=%s", conf.Namespace, conf.Name, message)
		_, err = s.k8sClient.CoreV1().ConfigMaps(conf.Namespace).Update(ctx, conf, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		confName := s.getConfigMapName(key)
		ns := s.getConfigMapNamespace(key)
		log.Error().Msgf("Unable to update configmap [%s/%s] %v.", ns, confName, retryErr)
	}
}

func (s *ConfigMapStore) GetLastEvent(ctx context.Context, key StoreKey) string {
	conf, err := s.GetConfigMap(ctx, key)
	if err != nil {
		return ""
	}
	return conf.Data[string(service.HookEvent)]
}
