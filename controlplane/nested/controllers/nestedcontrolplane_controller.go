/*
Copyright 2020 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"
	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-openstack/api/v1alpha5"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
)

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrollermanagers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete.
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete.

// NestedControlPlaneReconciler reconciles a NestedControlPlane object.
type NestedControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// SetupWithManager will configure the controller with the manager.
func (r *NestedControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.NestedControlPlane{}).
		Owns(&controlplanev1.NestedEtcd{}).
		Owns(&controlplanev1.NestedAPIServer{}).
		Owns(&controlplanev1.NestedControllerManager{}).
		Complete(r)
}

// Reconcile is ths main process which will handle updating the NCP.
func (r *NestedControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedcontrolplane", req.NamespacedName)
	log.Info("Reconciling NestedControlPlane...")
	// Fetch the NestedControlPlane
	ncp := &controlplanev1.NestedControlPlane{}
	if err := r.Get(ctx, req.NamespacedName, ncp); err != nil {
		// check for not found and don't requeue
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// If there are errors we should retry
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the cluster object
	cluster, err := ncp.GetOwnerCluster(ctx, r.Client)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{Requeue: true}, err
	}
	log = log.WithValues("cluster", cluster.Name)

	if annotations.IsPaused(cluster, ncp) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(ncp, r.Client)
	if err != nil {
		log.Error(err, "Failed to configure the patch helper")
		return ctrl.Result{Requeue: true}, nil
	}

	if !controllerutil.ContainsFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer) {
		controllerutil.AddFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer)

		// patch and return right away instead of reusing the main defer,
		// because the main defer may take too much time to get cluster status
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}
		if err := patchHelper.Patch(ctx, ncp, patchOpts...); err != nil {
			log.Error(err, "Failed to patch NestedControlPlane to add finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// TODO(christopherhein) handle deletion
	if !ncp.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion reconciliation loop.
		return r.reconcileDelete(ctx, log, ncp)
	}

	// Handle normal reconciliation loop.
	return r.reconcile(ctx, log, req, cluster, ncp)
}

// reconcileDelete will delete the control plane and all it's nestedcomponents.
func (r *NestedControlPlaneReconciler) reconcileDelete(ctx context.Context, log logr.Logger, ncp *controlplanev1.NestedControlPlane) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(ncp, r.Client)
	if err != nil {
		log.Error(err, "Failed to configure the patch helper")
		return ctrl.Result{Requeue: true}, nil
	}

	if controllerutil.ContainsFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer) {
		controllerutil.RemoveFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer)

		// patch and return right away instead of reusing the main defer,
		// because the main defer may take too much time to get cluster status
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}
		if err := patchHelper.Patch(ctx, ncp, patchOpts...); err != nil {
			log.Error(err, "Failed to patch NestedControlPlane to remove finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcile will handle all "normal" NCP reconciles this means create/update actions.
func (r *NestedControlPlaneReconciler) reconcile(ctx context.Context, log logr.Logger, req ctrl.Request, cluster *clusterv1.Cluster, ncp *controlplanev1.NestedControlPlane) (res ctrl.Result, reterr error) {
	log.Info("Reconcile NestedControlPlane")

	etcd := &druidv1alpha1.Etcd{}

	if err := r.Client.Get(ctx, req.NamespacedName, etcd); err != nil {
		if apierrors.IsNotFound(err) {
			etcd.Name = ncp.Name
			etcd.Namespace = ncp.Namespace
			etcd.Spec = druidv1alpha1.EtcdSpec{}
			if err := r.Client.Create(ctx, etcd); err != nil {
				log.Info("Fail to create etcd cluster")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: waitForResourceReady}, nil
		}
		return ctrl.Result{}, err
	}
	if !*etcd.Status.Ready {
		log.Info("Wait for etcd cluster ready")
		return ctrl.Result{Requeue: true}, nil
	}

	//get Data Store , create if not found   --> check Data Store (Kamaji)

	ds := &kamajiv1alpha1.DataStore{}
	if err := r.Client.Get(ctx, req.NamespacedName, ds); err != nil {
		if apierrors.IsNotFound(err) {
			ds.Name = ncp.Name
			ds.Namespace = ncp.Namespace
			ds.Spec = kamajiv1alpha1.DataStoreSpec{}
			if err := r.Client.Create(ctx, ds); err != nil {
				log.Info("Fail to create data store")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	//get Tenant Control Plane , create if not found   --> check Tenant Control Plane (Kamaji)
	// Set Status Initialized
	tcp := &kamajiv1alpha1.TenantControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, tcp); err != nil {
		if apierrors.IsNotFound(err) {
			tcp.Name = ncp.Name
			tcp.Namespace = ncp.Namespace
			tcp.Spec = kamajiv1alpha1.TenantControlPlaneSpec{}
			if err := r.Client.Create(ctx, tcp); err != nil {
				log.Info("Fail to create data store")
				return ctrl.Result{}, err
			}
			ncp.Status.Initialized = true
			if err := r.Client.Status().Update(ctx, ncp); err != nil {
				return reconcile.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Set Control Plane Endpoint for Infra Provider
	// Set Status Ready
	if tcp.Status.ControlPlaneEndpoint != "" {
		infra, err := external.Get(ctx, r.Client, cluster.Spec.InfrastructureRef, cluster.Namespace)
		if err != nil {
			if apierrors.IsNotFound(errors.Cause(err)) {
				return ctrl.Result{RequeueAfter: waitForResourceReady}, nil
			}
			return ctrl.Result{}, err
		}
		infraName := getNestedString(infra.Object, "metadata", "name")
		infraKind := getNestedString(infra.Object, "metadata", "kind")
		if strings.Contains(strings.ToLower(infraKind), "openstack") {
			osc := &infrav1.OpenStackCluster{}
			oscKey := types.NamespacedName{Name: infraName, Namespace: ncp.Namespace}
			if err := r.Client.Get(ctx, oscKey, osc); err != nil {
				return ctrl.Result{}, err
			}
			osc.Spec.ControlPlaneEndpoint.Host = ""
			osc.Spec.ControlPlaneEndpoint.Port = 123
			if err := r.Client.Update(ctx, osc); err != nil {
				log.Info("Error when update control plane endpoint for infra provider")
				return ctrl.Result{}, err
			}

		} else if strings.Contains(strings.ToLower(infraKind), "docker") {
			// TODO (thangtv32) support docker infra for test enviroment
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, errors.New("Infra Provider is not support")
		}

		//TODO (thangtv32) reconcile expired kubeconfig

		ncp.Status.Ready = true
		if err := r.Client.Status().Update(ctx, ncp); err != nil {
			return reconcile.Result{}, err
		}
	}

	return ctrl.Result{}, nil

}

// reconcileKubeconfig will check if the control plane endpoint has been set
// and if so it will generate the KUBECONFIG or regenerate if it's expired.
func (r *NestedControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, cluster *clusterv1.Cluster, ncp *controlplanev1.NestedControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	endpoint := cluster.Spec.ControlPlaneEndpoint
	if endpoint.IsZero() {
		return ctrl.Result{}, nil
	}

	controllerOwnerRef := *metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	clusterName := util.ObjectKey(cluster)
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	switch {
	case apierrors.IsNotFound(err):
		createErr := kubeconfig.CreateSecretWithOwner(
			ctx,
			r.Client,
			clusterName,
			endpoint.String(),
			controllerOwnerRef,
		)
		if errors.Is(createErr, kubeconfig.ErrDependentCertificateNotFound) {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		// always return if we have just created in order to skip rotation checks
		return ctrl.Result{}, createErr
	case err != nil:
		return ctrl.Result{}, errors.Wrap(err, "failed to retrieve kubeconfig Secret")
	}

	// only do rotation on owned secrets
	if !util.IsControlledBy(configSecret, ncp) {
		return ctrl.Result{}, nil
	}

	needsRotation, err := kubeconfig.NeedsClientCertRotation(configSecret, certs.ClientCertificateRenewalDuration)
	if err != nil {
		return ctrl.Result{}, err
	}

	if needsRotation {
		log.Info("rotating kubeconfig secret")
		if err := kubeconfig.RegenerateSecret(ctx, r.Client, configSecret); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to regenerate kubeconfig")
		}
	}

	return ctrl.Result{}, nil
}

// reconcileControllerOwners will loop through any known nested components that
// aren't owned by a control plane yet and associate them.
func (r *NestedControlPlaneReconciler) reconcileControllerOwners(ctx context.Context, ncp *controlplanev1.NestedControlPlane, addOwners []client.Object) error {
	for _, component := range addOwners {
		if err := ctrl.SetControllerReference(ncp, component, r.Scheme); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
				continue
			}
			return err
		}

		if err := r.Update(ctx, component); err != nil {
			return err
		}
	}
	return nil
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, found, err := NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return val
}
func NestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	val, found, err := NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := val.(string)
	if !ok {
		return "", false, fmt.Errorf("%v accessor error: %v is of the type %T, expected string", jsonPath(fields), val, val)
	}
	return s, true, nil
}
func jsonPath(fields []string) string {
	return "." + strings.Join(fields, ".")
}

func NestedFieldNoCopy(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var val interface{} = obj

	for i, field := range fields {
		if val == nil {
			return nil, false, nil
		}
		if m, ok := val.(map[string]interface{}); ok {
			val, ok = m[field]
			if !ok {
				return nil, false, nil
			}
		} else {
			return nil, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected map[string]interface{}", jsonPath(fields[:i+1]), val, val)
		}
	}
	return val, true, nil
}
