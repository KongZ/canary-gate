package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/KongZ/canary-gate/noti"
	"github.com/KongZ/canary-gate/service"
	"github.com/KongZ/canary-gate/store"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// CanaryWebhookPayload holds the deployment info and metadata sent to webhooks
type CanaryWebhookPayload struct {
	// Name of the canary
	Name string `json:"name"`

	// Namespace of the canary
	Namespace string `json:"namespace"`

	// Phase of the canary analysis
	Phase service.Phase `json:"phase"`

	// Hash from the TrackedConfigs and LastAppliedSpec of the Canary.
	// Can be used to identify a Canary for a specific configuration of the
	// deployed resources.
	Checksum string `json:"checksum"`

	// Metadata (key-value pairs) for this webhook
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CanaryGatePayload holds the open/close gate request
type CanaryGatePayload struct {
	// Name of the canary
	Type service.HookType `json:"type"`

	// Name of the canary
	Name string `json:"name"`

	// Namespace of the canary
	Namespace string `json:"namespace"`
}

// CanaryGatePayload holds the open/close gate request
type CanaryGateStatus struct {
	// Name of the canary
	Type service.HookType `json:"type"`
	// Name of the canary
	Name string `json:"name"`
	// Namespace of the canary
	Namespace string `json:"namespace"`
	// Gate status
	Status string `json:"status"`
}

type FlaggerHandler struct {
	cmd   *cli.Command
	noti  noti.Client
	store store.Store
}

// StoreKey get store key name
func StoreKey(canary *CanaryWebhookPayload, hook service.HookType) string {
	return fmt.Sprintf("%s:%s:%s", canary.Namespace, canary.Name, hook)
}

// ConfirmRollout hooks are executed before scaling up the canary deployment and can be used for manual approval. The rollout is paused until the  returns a successful HTTP status code.
func (h *FlaggerHandler) ConfirmRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookConfirmRollout, canary)
		if h.noti == nil {
			if _, err := h.noti.SendMessages("Please confirm rollout action", service.HookConfirmRollout, createMeta(*canary)); err != nil {
				log.Error().Msgf("Error while sending message %v", err)
			}
		}
		h.responseWebhook(w, canary, service.HookConfirmRollout)
	})
}

// PreRollout hooks are executed before routing traffic to canary. The canary advancement is paused if a pre-rollout  fails and if the number of failures reach the threshold the canary will be rollback
func (h *FlaggerHandler) PreRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving pre-rollout request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookPreRollout, canary)
		h.responseWebhook(w, canary, service.HookPreRollout)
	})
}

// Rollout hooks are executed during the analysis on each iteration before the metric checks. If a rollout call fails the canary advancement is paused and eventfully rolled back.
func (h *FlaggerHandler) Rollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving rollout request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookRollout, canary)
		h.responseWebhook(w, canary, service.HookRollout)
	})
}

// ConfirmTrafficIncrease hooks are executed right before the weight on the canary is increased. The canary advancement is paused until this returns HTTP 200.
func (h *FlaggerHandler) ConfirmTrafficIncrease() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		log.Info().Msgf("Received [confirm-traffic-increase][phase=%s][id=%s] %s [%#v]", canary.Phase, canary.Checksum, h.createWebhookKey(canary), canary.Metadata)
		h.responseWebhook(w, canary, service.HookConfirmTrafficIncrease)
	})
}

// ConfirmPromotion hooks are executed before the promotion step. The canary promotion is paused until the hooks return HTTP 200. While the promotion is paused, Flagger will continue to run the metrics checks and rollout hooks.
func (h *FlaggerHandler) ConfirmPromotion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookConfirmPromotion, canary)
		h.responseWebhook(w, canary, service.HookConfirmPromotion)
	})
}

// PostRollout hooks are executed after the canary has been promoted or rolled back. If a post rollout  fails the error is logged.
func (h *FlaggerHandler) PostRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		log.Info().Msgf("Received [post-rollout][phase=%s][id=%s] %s [%#v]", canary.Phase, canary.Checksum, h.createWebhookKey(canary), canary.Metadata)
		h.responseWebhook(w, canary, service.HookPostRollout)
	})
}

// Rollback hooks are executed while a canary deployment is in either Progressing or Waiting status. This provides the ability to rollback during analysis or while waiting for a confirmation. If a rollback  returns a successful HTTP status code, Flagger will stop the analysis and mark the canary release as failed.
func (h *FlaggerHandler) Rollback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving rollback request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookRollback, canary)
		h.responseWebhook(w, canary, service.HookRollback)
		// w.WriteHeader(http.StatusForbidden)
	})
}

func NewHandler(cmd *cli.Command, noti noti.Client, store store.Store) FlaggerHandler {
	handler := FlaggerHandler{
		cmd:   cmd,
		noti:  noti,
		store: store,
	}
	return handler
}

// Event hooks are executed every time Flagger emits a Kubernetes event. When configured, every action that Flagger takes during a canary deployment will be sent as JSON via an HTTP POST request
func (h *FlaggerHandler) Event() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.logEvent(service.HookEvent, canary)
		// h.noti.SendMessages()
	})
}

// OpenGate set gate open
func (h *FlaggerHandler) OpenGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.store.GateOpen(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gate.Type})
		h.responseAPI(w, gate, store.GATE_OPEN)
	})
}

// CloseGate set gate close
func (h *FlaggerHandler) CloseGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		h.store.GateClose(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gate.Type})
		h.responseAPI(w, gate, store.GATE_CLOSE)
	})
}

// StatusGate get gate status
func (h *FlaggerHandler) StatusGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			badRequest(w, err)
			return
		}
		var gateTypes []service.HookType
		if gate.Type == service.HookAll {
			gateTypes = []service.HookType{
				service.HookConfirmRollout,
				service.HookPreRollout,
				service.HookRollout,
				service.HookConfirmTrafficIncrease,
				service.HookConfirmPromotion,
				service.HookPostRollout,
				service.HookRollback,
			}
		} else {
			gateTypes = []service.HookType{gate.Type}
		}
		gateResponseMap := make(map[string][]CanaryGateStatus)
		for _, gt := range gateTypes {
			status := store.GateStatus(h.store.IsGateOpen(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gt}))
			log.Debug().Msgf("%s %s=%s", h.createKey(gate.Namespace, gate.Name), gt, status)
			h.createResponse(gateResponseMap, gate.Namespace, gate.Name, gt, status)
		}
		w.WriteHeader(http.StatusOK)
		writePayload(w, &gateResponseMap)
	})
}

func (h *FlaggerHandler) createWebhookKey(gate *CanaryWebhookPayload) string {
	return h.createKey(gate.Namespace, gate.Name)
}

func (h *FlaggerHandler) createKey(namespace string, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (h *FlaggerHandler) createResponse(result map[string][]CanaryGateStatus, namespace string, name string, t service.HookType, status string) {
	key := h.createKey(namespace, name)
	gateStatus := CanaryGateStatus{
		Type:      t,
		Name:      name,
		Namespace: namespace,
		Status:    status,
	}
	result[key] = append(result[key], gateStatus)
}

func (h *FlaggerHandler) responseAPI(w http.ResponseWriter, gate *CanaryGatePayload, status string) {
	gateResponseMap := make(map[string][]CanaryGateStatus)
	h.createResponse(gateResponseMap, gate.Namespace, gate.Name, gate.Type, status)
	w.WriteHeader(http.StatusOK)
	writePayload(w, &gateResponseMap)
}

func (h *FlaggerHandler) responseWebhook(w http.ResponseWriter, canary *CanaryWebhookPayload, hookType service.HookType) {
	approved := h.store.IsGateOpen(store.StoreKey{Namespace: canary.Namespace, Name: canary.Name, Type: hookType})
	if approved {
		log.Info().Msgf("%s:%s of [%s] is approved", canary.Namespace, canary.Name, hookType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Approved")); err != nil {
			log.Error().Msgf("Error while writing body %v", err)
		}
	} else {
		log.Info().Msgf("%s:%s of [%s] is rejected", canary.Namespace, canary.Name, hookType)
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte("Forbidden")); err != nil {
			log.Error().Msgf("Error while writing body %v", err)
		}
	}
}

func (h *FlaggerHandler) logEvent(hook service.HookType, canary *CanaryWebhookPayload) {
	var metadataBuilder strings.Builder
	for k, v := range canary.Metadata {
		if k != "eventMessage" {
			if metadataBuilder.Len() > 0 {
				metadataBuilder.WriteString(", ")
			}
			metadataBuilder.WriteString(fmt.Sprintf("%s=%s", k, v))
		}
	}
	log.Info().Msgf("Received [%s][phase=%s][id=%s] %s %s meta=[%s]", hook, canary.Phase, canary.Checksum, h.createWebhookKey(canary), canary.Metadata["eventMessage"], metadataBuilder.String())
	if h.store != nil {
		stor, ok := h.store.(*store.CanaryGateStore)
		if ok {
			stor.UpdateCanaryGateStatus(context.TODO(), store.StoreKey{Namespace: canary.Namespace, Name: canary.Name}, string(canary.Phase), metadataBuilder.String())
		}
	}

}

func createMeta(canary CanaryWebhookPayload) map[string]string {
	m := map[string]string{
		"name":      canary.Name,
		"namespace": canary.Namespace,
	}
	maps.Copy(m, canary.Metadata)
	return m
}

func badRequest(w http.ResponseWriter, err error) {
	log.Error().Msgf("Reading the request body failed %v", err)
	w.WriteHeader(http.StatusBadRequest)
}

func readPayload[I any](r *http.Request, i I) (*I, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return &i, err
	}
	defer r.Body.Close()
	err = json.Unmarshal(body, &i)
	if err != nil {
		return &i, err
	}
	return &i, nil
}

func writePayload[I any](w http.ResponseWriter, payload *I) {
	r, err := json.Marshal(payload)
	if err != nil {
		log.Error().Msgf("Error while read payload %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if _, err := w.Write(r); err != nil {
		log.Error().Msgf("Error while writing body %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
