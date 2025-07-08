package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	flaggerv1beta1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	piggysecvalpha1 "github.com/KongZ/canary-gate/api/v1alpha1"
	"github.com/KongZ/canary-gate/service"
)

// CanaryGateReconciler reconciles a CanaryGate object
type CanaryGateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=piggysec.com,resources=canarygates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=piggysec.com,resources=canarygates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=piggysec.com,resources=canarygates/finalizers,verbs=update
// +kubebuilder:rbac:groups=flagger.app,resources=canaries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (r *CanaryGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// Fetch the CanaryGate crd
	var canaryGate piggysecvalpha1.CanaryGate
	if err := r.Get(ctx, req.NamespacedName, &canaryGate); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info().Msg("CanaryGate resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error().Err(err).Msg("Failed to get CanaryGate")
		return ctrl.Result{}, err
	}

	// Deserialize the raw Flagger spec into a Flagger CanarySpec struct
	// This gives us typed access to the spec while preserving all other fields.
	var flaggerSpec flaggerv1beta1.CanarySpec
	if err := json.Unmarshal(canaryGate.Spec.Flagger.Raw, &flaggerSpec); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal Flagger spec from CanaryGate")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	endpoint := os.Getenv("CANARY_GATE_ENDPOINT")

	// Ensure the Analysis field is not nil
	if flaggerSpec.Analysis == nil {
		flaggerSpec.Analysis = &flaggerv1beta1.CanaryAnalysis{}
	}

	defaultMetadata := &map[string]string{
		"gate_name":      canaryGate.Name,
		"gate_namespace": canaryGate.Namespace,
	}

	// Prepend our controlled webhook.
	flaggerSpec.Analysis.Webhooks = []flaggerv1beta1.CanaryWebhook{
		{
			Name:     string(service.HookConfirmRollout),
			Type:     flaggerv1beta1.ConfirmRolloutHook,
			URL:      fmt.Sprintf("%s/confirm-rollout", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookPreRollout),
			Type:     flaggerv1beta1.PreRolloutHook,
			URL:      fmt.Sprintf("%s/pre-rollout", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookRollout),
			Type:     flaggerv1beta1.RolloutHook,
			URL:      fmt.Sprintf("%s/rollout", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookConfirmTrafficIncrease),
			Type:     flaggerv1beta1.ConfirmTrafficIncreaseHook,
			URL:      fmt.Sprintf("%s/confirm-traffic-increase", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookConfirmPromotion),
			Type:     flaggerv1beta1.ConfirmPromotionHook,
			URL:      fmt.Sprintf("%s/confirm-promotion", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookPostRollout),
			Type:     flaggerv1beta1.PostRolloutHook,
			URL:      fmt.Sprintf("%s/post-rollout", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookRollback),
			Type:     flaggerv1beta1.RollbackHook,
			URL:      fmt.Sprintf("%s/rollback", endpoint),
			Metadata: defaultMetadata,
		},
		{
			Name:     string(service.HookEvent),
			Type:     flaggerv1beta1.EventHook,
			URL:      fmt.Sprintf("%s/event", endpoint),
			Metadata: defaultMetadata,
		},
	}

	// Construct the Canary object
	canary := &flaggerv1beta1.Canary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      canaryGate.Spec.Target.Name,
			Namespace: canaryGate.Spec.Target.Namespace, // Create Canary in the target namespace
		},
		Spec: flaggerSpec, // Assign the modified spec
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, canary, func() error {
		canary.Spec = flaggerSpec
		return nil
		// Return SetControllerReference for makeing reference to Canary then when CanaryGate is deleted, Canary will be deleted too
		// return controllerutil.SetControllerReference(&canaryGate, canary, r.Scheme)
	})

	log.Trace().
		Str("namespace", canaryGate.Spec.Target.Namespace).
		Str("name", canaryGate.Spec.Target.Name).
		Msg("Successfully injected custom webhook into Canary spec")

	if err != nil {
		log.Error().Err(err).Msg("Failed to create or update Canary resource")
		r.Recorder.Event(&canaryGate, corev1.EventTypeWarning, "ReconcileFailed", err.Error())
		return ctrl.Result{}, err
	}

	if result != controllerutil.OperationResultNone {
		msg := fmt.Sprintf("Canary resource %s successfully", result)
		log.Info().Str("operation", string(result)).Msg(msg)
		r.Recorder.Event(&canaryGate, corev1.EventTypeNormal, "CanaryReconciled", msg)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CanaryGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&piggysecvalpha1.CanaryGate{}). // Watch for CanaryGate resources
		Owns(&flaggerv1beta1.Canary{}).     // Also watch for Canaries owned by a CanaryGate
		Complete(r)
}
