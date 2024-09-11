package store

import (
	"context"
	"fmt"
	"strconv"
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
}

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

// StoreKey get store key name
func (s *ConfigMapStore) getConfigMapName(key StoreKey) string {
	return fmt.Sprintf("%s-%s", key.Name, ConfigMapSuffix)
}

// StoreKey get store key name
func (s *ConfigMapStore) createConfigMap(key StoreKey) *corev1.ConfigMap {
	confName := s.getConfigMapName(key)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: confName},
		Data:       map[string]string{},
	}
	configMap.Data[string(key.Type)] = strconv.FormatBool(defaultValue(key))
	_, err := s.k8sClient.CoreV1().ConfigMaps(key.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		log.Error().Msgf("Error while creating configmap [%s] %v. Gate [%s] is set to [%s]", confName, err, key, defaultText(key))
	}
	return configMap
}

func (s *ConfigMapStore) updateGate(key StoreKey, val bool) {
	confName := s.getConfigMapName(key)
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// defer cancel()
		conf, err := s.k8sClient.CoreV1().ConfigMaps(key.Namespace).Get(context.TODO(), confName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warn().Msgf("Unable to load configmap [%s].", confName)
				conf = s.createConfigMap(key)
			} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
				log.Error().Msgf("Error to load configmap [%s] %v.", confName, statusError.ErrStatus.Message)
				return err
			}
		}
		conf.Data[string(key.Type)] = strconv.FormatBool(val)
		_, err = s.k8sClient.CoreV1().ConfigMaps(key.Namespace).Update(context.TODO(), conf, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		log.Error().Msgf("Unable to update configmap [%s] %v.", confName, retryErr)
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
	conf, err := s.k8sClient.CoreV1().ConfigMaps(key.Namespace).Get(context.TODO(), confName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warn().Msgf("Unable to load configmap [%s]. Gate [%s] is set to [%s]", confName, key, defaultText(key))
			conf = s.createConfigMap(key)
		} else if statusError, isStatus := err.(*k8serrors.StatusError); isStatus {
			log.Error().Msgf("Error to load configmap [%s] %v. Gate [%s] is set to [%s]", confName, statusError.ErrStatus.Message, key, defaultText(key))
			return defaultValue(key)
		}
	}
	val, ok := conf.Data[string(key.Type)]
	if ok {
		boolValue, err := strconv.ParseBool(val)
		if err != nil {
			return defaultValue(key)
		}
		return boolValue
	}
	return defaultValue(key)
}
