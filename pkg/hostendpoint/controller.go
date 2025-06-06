package hostendpoint

import (
	"context"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/infrastructure-io/topohub/pkg/config"
	topohubv1beta1 "github.com/infrastructure-io/topohub/pkg/k8s/apis/topohub.infrastructure.io/v1beta1"
	"github.com/infrastructure-io/topohub/pkg/log"
)

// HostEndpointReconciler reconciles a HostEndpoint object
type HostEndpointReconciler struct {
	client     client.Client
	kubeClient kubernetes.Interface
	config     *config.AgentConfig
	log        *zap.SugaredLogger
}

// NewHostEndpointReconciler creates a new HostEndpoint reconciler
func NewHostEndpointReconciler(mgr ctrl.Manager, kubeClient kubernetes.Interface, config *config.AgentConfig) (*HostEndpointReconciler, error) {
	return &HostEndpointReconciler{
		client:     mgr.GetClient(),
		kubeClient: kubeClient,
		config:     config,
		log:        log.Logger.Named("hostendpointReconcile"),
	}, nil
}

// 只有 leader 才会执行 Reconcile
// Reconcile handles the reconciliation of HostEndpoint objects
func (r *HostEndpointReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With("hostendpoint", req.Name)

	// 获取 HostEndpoint
	hostEndpoint := &topohubv1beta1.HostEndpoint{}
	if err := r.client.Get(ctx, req.NamespacedName, hostEndpoint); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HostEndpoint not found, ignoring")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get HostEndpoint")
		return reconcile.Result{}, err
	}

	// 处理 HostEndpoint
	if err := r.handleHostEndpoint(ctx, hostEndpoint, logger); err != nil {
		logger.Error(err, "Failed to handle HostEndpoint")
		return reconcile.Result{
			RequeueAfter: time.Second * 2,
		}, err
	}

	return reconcile.Result{}, nil
}

// 根据 HostEndpoint ，同步更新对应的 RedfishStatus
func (r *HostEndpointReconciler) handleHostEndpoint(ctx context.Context, hostEndpoint *topohubv1beta1.HostEndpoint, logger *zap.SugaredLogger) error {
	name := hostEndpoint.Name
	logger.Debugf("Processing HostEndpoint %s (IP: %s)", name, hostEndpoint.Spec.IPAddr)

	// Try to get existing RedfishStatus
	existing := &topohubv1beta1.RedfishStatus{}
	err := r.client.Get(ctx, client.ObjectKey{Name: name}, existing)
	if err == nil {
		// RedfishStatus exists, check if spec changed
		if specEqual(existing.Status.Basic, hostEndpoint.Spec) {
			logger.Debugf("RedfishStatus %s exists with same spec, no update needed", name)
			return nil
		}

		// Spec changed, update the object
		logger.Infof("Updating RedfishStatus %s due to spec change", name)

		// Create a copy of the existing object to avoid modifying the cache
		updated := existing.DeepCopy()
		updated.Status.LastUpdateTime = time.Now().UTC().Format(time.RFC3339)
		updated.Status.Basic = topohubv1beta1.BasicInfo{
			Type:   topohubv1beta1.HostTypeEndpoint,
			IpAddr: hostEndpoint.Spec.IPAddr,
			Https:  true,
			Port:   443,
		}
		if hostEndpoint.Spec.SecretName != nil {
			updated.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
		}
		if hostEndpoint.Spec.SecretNamespace != nil {
			updated.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
		}
		if hostEndpoint.Spec.HTTPS != nil {
			updated.Status.Basic.Https = *hostEndpoint.Spec.HTTPS
		}
		if hostEndpoint.Spec.Port != nil {
			updated.Status.Basic.Port = *hostEndpoint.Spec.Port
		}

		if err := r.client.Update(ctx, updated); err != nil {
			if errors.IsConflict(err) {
				logger.Debugf("Conflict updating RedfishStatus %s, will retry", name)
				return err
			}
			logger.Errorf("Failed to update RedfishStatus %s: %v", name, err)
			return err
		}
		logger.Infof("Successfully updated RedfishStatus %s", name)
		logger.Debugf("Updated RedfishStatus details - IP: %s, Secret: %s/%s, Port: %d",
			updated.Status.Basic.IpAddr,
			updated.Status.Basic.SecretNamespace,
			updated.Status.Basic.SecretName,
			updated.Status.Basic.Port)
		return nil
	}

	if !errors.IsNotFound(err) {
		logger.Errorf("Failed to get RedfishStatus %s: %v", name, err)
		return err
	}

	// RedfishStatus doesn't exist, create new one
	redfishStatus := &topohubv1beta1.RedfishStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				topohubv1beta1.LabelIPAddr:     hostEndpoint.Spec.IPAddr,
				topohubv1beta1.LabelClientMode: topohubv1beta1.HostTypeEndpoint,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         topohubv1beta1.APIVersion,
					Kind:               topohubv1beta1.KindHostEndpoint,
					Name:               hostEndpoint.Name,
					UID:                hostEndpoint.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
	}

	// RedfishStatus doesn't exist, create new one
	// IMPORTANT: When creating a new RedfishStatus, we must follow a two-step process:
	// 1. First create the resource with only metadata (no status). This is because
	//    the Kubernetes API server does not allow setting status during creation.
	// 2. Then update the status separately using UpdateStatus. If we try to set
	//    status during creation, the status will be silently ignored, leading to
	//    a RedfishStatus without any status information until the next reconciliation.
	logger.Debugf("Creating new RedfishStatus %s", name)
	if err := r.client.Create(ctx, redfishStatus); err != nil {
		logger.Errorf("Failed to create RedfishStatus %s: %v", name, err)
		return err
	}

	// Get the latest version of the resource after creation
	// if err := r.client.Get(ctx, client.ObjectKey{Name: name}, redfishStatus); err != nil {
	// 	logger.Errorf("Failed to get latest version of RedfishStatus %s: %v", name, err)
	// 	return err
	// }

	// Now update the status using the latest version
	clusterName := ""
	if hostEndpoint.Spec.ClusterName != nil {
		clusterName = *hostEndpoint.Spec.ClusterName
	}

	redfishStatus.Status = topohubv1beta1.RedfishStatusStatus{
		Healthy:        false,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		Basic: topohubv1beta1.BasicInfo{
			Type:        topohubv1beta1.HostTypeEndpoint,
			IpAddr:      hostEndpoint.Spec.IPAddr,
			Https:       true,
			Port:        443,
			ClusterName: clusterName,
		},
		Info: map[string]string{},
		Log: topohubv1beta1.LogStruct{
			TotalLogAccount:   0,
			WarningLogAccount: 0,
			LastestLog:        nil,
			LastestWarningLog: nil,
		},
	}
	if hostEndpoint.Spec.SecretName != nil {
		redfishStatus.Status.Basic.SecretName = *hostEndpoint.Spec.SecretName
	}
	if hostEndpoint.Spec.SecretNamespace != nil {
		redfishStatus.Status.Basic.SecretNamespace = *hostEndpoint.Spec.SecretNamespace
	}
	if hostEndpoint.Spec.HTTPS != nil {
		redfishStatus.Status.Basic.Https = *hostEndpoint.Spec.HTTPS
	}
	if hostEndpoint.Spec.Port != nil {
		redfishStatus.Status.Basic.Port = *hostEndpoint.Spec.Port
	}

	if err := r.client.Status().Update(ctx, redfishStatus); err != nil {
		logger.Errorf("Failed to update status of RedfishStatus %s: %v", name, err)
		return err
	}

	logger.Infof("Successfully created RedfishStatus %s", name)
	logger.Debugf("RedfishStatus details - IP: %s, Secret: %s/%s, Port: %d",
		redfishStatus.Status.Basic.IpAddr,
		redfishStatus.Status.Basic.SecretNamespace,
		redfishStatus.Status.Basic.SecretName,
		redfishStatus.Status.Basic.Port)
	return nil
}

// specEqual checks if the RedfishStatus basic info matches the HostEndpoint spec
func specEqual(basic topohubv1beta1.BasicInfo, spec topohubv1beta1.HostEndpointSpec) bool {
	clusterName := ""
	if spec.ClusterName != nil {
		clusterName = *spec.ClusterName
	}
	t1 := false
	if spec.SecretName != nil && basic.SecretName == *spec.SecretName {
		t1 = true
	}
	t2 := false
	if spec.SecretNamespace != nil && basic.SecretNamespace == *spec.SecretNamespace {
		t2 = true
	}
	t3 := false
	if spec.HTTPS != nil && basic.Https == *spec.HTTPS {
		t3 = true
	}
	t4 := false
	if spec.Port != nil && basic.Port == *spec.Port {
		t4 = true
	}

	return basic.IpAddr == spec.IPAddr &&
		t1 &&
		t2 &&
		t3 &&
		t4 &&
		clusterName == basic.ClusterName
}

// SetupWithManager sets up the controller with the Manager
func (r *HostEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topohubv1beta1.HostEndpoint{}).
		Complete(r)
}
