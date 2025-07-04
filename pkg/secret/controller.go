package secret

import (
	"context"

	"go.uber.org/zap"

	"github.com/infrastructure-io/topohub/pkg/log"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/infrastructure-io/topohub/pkg/config"
	"github.com/infrastructure-io/topohub/pkg/redfishstatus"
	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct {
	client                  client.Client
	config                  *config.AgentConfig
	redfishStatusController redfishstatus.RedfishStatusController
	log                     *zap.SugaredLogger
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewSecretReconciler(mgr ctrl.Manager, config *config.AgentConfig, redfishStatusController redfishstatus.RedfishStatusController) (*SecretReconciler, error) {
	return &SecretReconciler{
		client:                  mgr.GetClient(),
		config:                  config,
		redfishStatusController: redfishStatusController,
		log:                     log.Logger.Named("secretReconciler"),
	}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create predicate to filter secrets by label
	redfishSecretPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		labels := obj.GetLabels()
		if labels == nil {
			return false
		}
		// Check if the secret has the topohub.io/secret-credential label (any value)
		if _, exists := labels["topohub.io/secret-credential"]; exists {
			return true
		}
		return false
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(redfishSecretPredicate).
		Complete(r)
}

// 监控 secret 的变更，同步给 redfishStatus 控制器，便于 更新 redfish 认证信息
// Reconcile handles the reconciliation of HostEndpoint objects
func (r *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("secret", req.Name)

	logger.Debugf("Reconciling Secret %s", req.Name)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			logger.Debugf("Secret not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get Secret")
		return reconcile.Result{}, err
	}

	if _, ok := secret.Data["username"]; !ok {
		return reconcile.Result{}, nil
	}
	if _, ok := secret.Data["password"]; !ok {
		return reconcile.Result{}, nil
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	logger.Debugf("retrieved new secret data for %s/%s", secret.Namespace, secret.Name)
	r.redfishStatusController.UpdateSecret(secret.Name, secret.Namespace, username, password)

	return reconcile.Result{}, nil
}
