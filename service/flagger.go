package service

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
	// HookAll convenience type to represents all hooks
	HookAll HookType = "all"
)

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

const (
	// a deployment name
	MetaName string = "name"
	// a deployment namespace
	MetaNamespace string = "namespace"
	// a deployment cluster
	MetaCluster string = "cluster"
	// a name of response user
	MetaUser string = "user"
)
