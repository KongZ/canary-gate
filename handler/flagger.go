package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"

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
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [confirm-rollout] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		if _, err := h.noti.SendMessages("Please confirm rollout action", service.HookConfirmRollout, createMeta(*canary)); err != nil {
			log.Error().Msgf("Error while sending message %v", err)
		}
		h.response(w, r, canary, service.HookConfirmRollout)
	})
}

// PreRollout hooks are executed before routing traffic to canary. The canary advancement is paused if a pre-rollout  fails and if the number of failures reach the threshold the canary will be rollback
func (h *FlaggerHandler) PreRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving pre-rollout request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [pre-rollout] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookPreRollout)
	})
}

// Rollout hooks are executed during the analysis on each iteration before the metric checks. If a rollout call fails the canary advancement is paused and eventfully rolled back.
func (h *FlaggerHandler) Rollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving rollout request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [rollout] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookRollout)
	})
}

// ConfirmTrafficIncrease hooks are executed right before the weight on the canary is increased. The canary advancement is paused until this returns HTTP 200.
func (h *FlaggerHandler) ConfirmTrafficIncrease() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [confirm-traffic-increase] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookConfirmTrafficIncrease)
	})
}

// ConfirmPromotion hooks are executed before the promotion step. The canary promotion is paused until the hooks return HTTP 200. While the promotion is paused, Flagger will continue to run the metrics checks and rollout hooks.
func (h *FlaggerHandler) ConfirmPromotion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [confirm-promotion] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookConfirmPromotion)
	})
}

// PostRollout hooks are executed after the canary has been promoted or rolled back. If a post rollout  fails the error is logged.
func (h *FlaggerHandler) PostRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [post-rollout] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookPostRollout)
	})
}

// Rollback hooks are executed while a canary deployment is in either Progressing or Waiting status. This provides the ability to rollback during analysis or while waiting for a confirmation. If a rollback  returns a successful HTTP status code, Flagger will stop the analysis and mark the canary release as failed.
func (h *FlaggerHandler) Rollback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msgf("Receiving rollback request ...")
		canary, err := readPayload(r, CanaryWebhookPayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [rollback] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		h.response(w, r, canary, service.HookRollback)
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
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received [event] %s:%s event %s [%s][%+v]", canary.Name, canary.Namespace, canary.Phase, canary.Checksum, canary.Metadata)
		// h.noti.SendMessages()
	})
}

// OpenGate set gate open
func (h *FlaggerHandler) OpenGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.store.GateOpen(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gate.Type})
		h.CliResponse(w, gate, "opened")
	})
}

// CloseGate set gate close
func (h *FlaggerHandler) CloseGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.store.GateClose(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gate.Type})
		h.CliResponse(w, gate, "closed")
	})
}

// StatusGate get gate status
func (h *FlaggerHandler) StatusGate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, err := readPayload(r, CanaryGatePayload{})
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		approved := h.store.IsGateOpen(store.StoreKey{Namespace: gate.Namespace, Name: gate.Name, Type: gate.Type})
		status := "closed"
		if approved {
			status = "opened"
		}
		h.CliResponse(w, gate, status)
	})
}

func (h *FlaggerHandler) CliResponse(w http.ResponseWriter, gate *CanaryGatePayload, status string) {
	response := CanaryGateStatus{
		Type:      gate.Type,
		Name:      gate.Name,
		Namespace: gate.Namespace,
		Status:    status,
	}
	w.WriteHeader(http.StatusOK)
	log.Info().Msgf("%s:%s of [%s] is %s", gate.Namespace, gate.Name, gate.Type, status)
	writePayload(w, &response)
}

func (h *FlaggerHandler) response(w http.ResponseWriter, r *http.Request, canary *CanaryWebhookPayload, hookType service.HookType) {
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

func createMeta(canary CanaryWebhookPayload) map[string]string {
	m := map[string]string{
		"name":      canary.Name,
		"namespace": canary.Namespace,
	}
	maps.Copy(m, canary.Metadata)
	return m
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
	r, err := json.Marshal(&payload)
	if err != nil {
		log.Error().Msgf("Error while read payload %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if _, err := w.Write(r); err != nil {
		log.Error().Msgf("Error while writing body %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
