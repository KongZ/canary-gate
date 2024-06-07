package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/KongZ/canary-gate/noti"
	"github.com/KongZ/canary-gate/store"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// HookType can be pre, post or during rollout
type HookType string

const (
	// HookRollout execute web during the canary analysis
	HookRollout HookType = "rollout"
	// HookPreRollout execute web before routing traffic to canary
	HookPreRollout HookType = "pre-rollout"
	// HookPostRollout execute web after the canary analysis
	HookPostRollout HookType = "post-rollout"
	// HookConfirmRollout halt canary analysis until web returns HTTP 200
	HookConfirmRollout HookType = "confirm-rollout"
	// HookConfirmPromotion halt canary promotion until web returns HTTP 200
	HookConfirmPromotion HookType = "confirm-promotion"
	// HookEvent dispatches Flagger events to the specified endpoint
	HookEvent HookType = "event"
	// HookRollback rollback canary analysis if web returns HTTP 200
	HookRollback HookType = "rollback"
	// HookConfirmTrafficIncrease increases traffic weight if web returns HTTP 200
	HookConfirmTrafficIncrease HookType = "confirm-traffic-increase"
)

// Phase is a label for the condition of a canary at the current time
type Phase string

const (
	// PhaseInitializing means the canary initializing is underway
	PhaseInitializing Phase = "Initializing"
	// PhaseInitialized means the primary deployment, hpa and ClusterIP services
	// have been created along with the service mesh or ingress objects
	PhaseInitialized Phase = "Initialized"
	// PhaseWaiting means the canary rollout is paused (waiting for confirmation to proceed)
	PhaseWaiting Phase = "Waiting"
	// PhaseProgressing means the canary analysis is underway
	PhaseProgressing Phase = "Progressing"
	// PhaseWaitingPromotion means the canary promotion is paused (waiting for confirmation to proceed)
	PhaseWaitingPromotion Phase = "WaitingPromotion"
	// PhasePromoting means the canary analysis is finished and the primary spec has been updated
	PhasePromoting Phase = "Promoting"
	// PhaseFinalising means the canary promotion is finished and traffic has been routed back to primary
	PhaseFinalising Phase = "Finalising"
	// PhaseSucceeded means the canary analysis has been successful
	// and the canary deployment has been promoted
	PhaseSucceeded Phase = "Succeeded"
	// PhaseFailed means the canary analysis failed
	// and the canary deployment has been scaled to zero
	PhaseFailed Phase = "Failed"
	// PhaseTerminating means the canary has been marked
	// for deletion and in the finalizing state
	PhaseTerminating Phase = "Terminating"
	// PhaseTerminated means the canary has been finalized
	// and successfully deleted
	PhaseTerminated Phase = "Terminated"
)

// CanaryWebhookPayload holds the deployment info and metadata sent to webhooks
type CanaryWebhookPayload struct {
	// Name of the canary
	Name string `json:"name"`

	// Namespace of the canary
	Namespace string `json:"namespace"`

	// Phase of the canary analysis
	Phase Phase `json:"phase"`

	// Hash from the TrackedConfigs and LastAppliedSpec of the Canary.
	// Can be used to identify a Canary for a specific configuration of the
	// deployed resources.
	Checksum string `json:"checksum"`

	// Metadata (key-value pairs) for this webhook
	Metadata map[string]string `json:"metadata,omitempty"`
}

type FlaggerHandler struct {
	cmd   *cli.Command
	noti  noti.Client
	store store.Store
}

// StoreKey get store key name
func StoreKey(canary *CanaryWebhookPayload) string {
	return fmt.Sprintf("%s:%s:%s", canary.Namespace, canary.Name, canary.Phase)
}

// ConfirmRollout hooks are executed before scaling up the canary deployment and can be used for manual approval. The rollout is paused until the  returns a successful HTTP status code.
func ConfirmRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving confirm-rollout request ...")
	})
}

// PreRollout hooks are executed before routing traffic to canary. The canary advancement is paused if a pre-rollout  fails and if the number of failures reach the threshold the canary will be rollback
func PreRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving pre-rollout request ...")
	})
}

// Rollout hooks are executed during the analysis on each iteration before the metric checks. If a rollout  call fails the canary advancement is paused and eventfully rolled back.
func Rollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving rollout request ...")
	})
}

// ConfirmTrafficIncrease hooks are executed right before the weight on the canary is increased. The canary advancement is paused until this  returns HTTP 200.
func ConfirmTrafficIncrease() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving confirm-traffic-increase request ...")
	})
}

// ConfirmPromotion hooks are executed before the promotion step. The canary promotion is paused until the hooks return HTTP 200. While the promotion is paused, Flagger will continue to run the metrics checks and rollout hooks.
func ConfirmPromotion() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving confirm-promotion request ...")
	})
}

// PostRollout hooks are executed after the canary has been promoted or rolled back. If a post rollout  fails the error is logged.
func PostRollout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving post-rollout request ...")
	})
}

// Rollback hooks are executed while a canary deployment is in either Progressing or Waiting status. This provides the ability to rollback during analysis or while waiting for a confirmation. If a rollback  returns a successful HTTP status code, Flagger will stop the analysis and mark the canary release as failed.
func Rollback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("Receiving rollback request ...")
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
		canary, err := h.readPayload(r)
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Info().Msgf("Received %s:%s event %s", canary.Name, canary.Namespace, canary.Phase)
		// h.noti.SendMessages()
	})
}

// Gate hooks are executed every time Flagger emits a Kubernetes event. When configured, every action that Flagger takes during a canary deployment will be sent as JSON via an HTTP POST request
func (h *FlaggerHandler) Gate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		canary, err := h.readPayload(r)
		if err != nil {
			log.Error().Msgf("Reading the request body failed %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		approved := h.store.IsGateOpen(StoreKey(canary))
		if approved {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Approved")); err != nil {
				log.Error().Msgf("Error while writing body %v", err)
			}
		} else {
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte("Forbidden")); err != nil {
				log.Error().Msgf("Error while writing body %v", err)
			}
		}
		log.Info().Msgf("%s:%s of %s is approved %v", canary.Name, canary.Namespace, canary.Phase, approved)
	})
}

func (h *FlaggerHandler) readPayload(r *http.Request) (*CanaryWebhookPayload, error) {
	canary := &CanaryWebhookPayload{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return canary, err
	}
	defer r.Body.Close()
	err = json.Unmarshal(body, canary)
	if err != nil {
		return canary, err
	}
	return canary, nil
}
