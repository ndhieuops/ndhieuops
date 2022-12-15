/*
Copyright 2022.

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	_ "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cloud "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/viettel-cloud"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "git.viettel.vn/cloud-native-cicd/kubernetes-engine/cluster-api-provider-viettel/api/v1"
)

// ViettelClusterReconciler reconciles a ViettelCluster object
type ViettelClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=infrastructure.git.viettel.vn,resources=viettelclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.git.viettel.vn,resources=viettelclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.git.viettel.vn,resources=viettelclusters/finalizers,verbs=update

func (r *ViettelClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, ers error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling Viettel Cluster... : ", "ViettelCluster", req.Namespace)
	var ViettelCluster = &infrav1.ViettelCluster{}
	err := r.Client.Get(ctx, req.NamespacedName, ViettelCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, ViettelCluster.ObjectMeta)
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	if annotations.IsPaused(cluster, ViettelCluster) {
		log.Info("ViettelCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(ViettelCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the ViettelCluster when exiting this function so we can persist any ViettelCluster changes.
	defer func() {
		if err := patchHelper.Patch(ctx, ViettelCluster); err != nil {
			if ers == nil {
				ers = errors.Wrapf(err, "error patching ViettelCluster %s/%s", ViettelCluster.Namespace, ViettelCluster.Name)
			}
		}
	}()

	// Authentication with cloud provider
	var vcs infrav1.ViettelClusterSpec
	cloudProvider, err := cloud.CreateViettelCloudProvider(vcs.ProjectID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("have trouble when authenticate with cloud provider", err)
	}

	if !ViettelCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, cloudProvider, patchHelper, cluster, ViettelCluster)
	}

	return r.reconcileNormal(ctx, log, cloudProvider, patchHelper, ViettelCluster)
}

func (r *ViettelClusterReconciler) reconcileDelete(ctx context.Context, log logr.Logger, cloudProvider cloud.ViettelCloudProvider, patchHelper *patch.Helper, cluster *clusterv1.Cluster, ViettelCluster *infrav1.ViettelCluster) (ctrl.Result, error) {
	log.Info("Reconciling Cluster delete : ", "Cluster ", cluster.Name, "with namespace ", cluster.Namespace)

	if err := patchHelper.Patch(ctx, ViettelCluster); err != nil {
		return ctrl.Result{}, err
	}

	ProjectID, _ := uuid.Parse(cloudProvider.CloudProjectID)
	err := cloudProvider.Cloud.ReconcileDeleteLB(r.Log, ctx, ViettelCluster, ProjectID)
	if err != nil {
		r.Log.Info("Fail to delete LoadBalancer")
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ViettelClusterReconciler) reconcileNormal(ctx context.Context, log logr.Logger, cloudProvider cloud.ViettelCloudProvider, patchHelper *patch.Helper, ViettelCluster *infrav1.ViettelCluster) (ctrl.Result, error) {
	log.Info("Reconciling Cluster")
	// Register the finalizer immediately to avoid orphaning OpenStack resources on delete
	if err := patchHelper.Patch(ctx, ViettelCluster); err != nil {
		return reconcile.Result{}, err
	}

	// start reconcile vpc for CAPV
	ProjectID, _ := uuid.Parse(cloudProvider.CloudProjectID)
	var vcs infrav1.ViettelClusterSpec
	vpc, err := cloudProvider.Cloud.ReconcileVpc(log, ctx, ViettelCluster, vcs, ProjectID)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("fail when reconcile vpc %s", err)
	}

	err = cloudProvider.Cloud.ReconcileSubnet(log, ctx, ViettelCluster, vcs, ProjectID)
	if err != nil {
		r.Log.Info("Fail reconcile VPC")
		return reconcile.Result{}, err
	}

	err = cloudProvider.Cloud.ReconcileLB(log, ctx, ViettelCluster, ProjectID, vpc, vcs)
	if err != nil {
		r.Log.Info("Fail reconcile LoadBalancer")
		return reconcile.Result{}, err
	}
	ViettelCluster.Status.Ready = true
	ViettelCluster.Status.FailureMessage = ""
	ViettelCluster.Status.FailureReason = nil
	r.Log.Info("Reconciled Cluster create successfully")
	return reconcile.Result{}, nil
}

func (r *ViettelClusterReconciler) reconcileTimeout(cluster *clusterv1.Cluster, ViettelCluster *infrav1.ViettelCluster) error {
	now := time.Now()
	creationTime := cluster.CreationTimestamp.Time
	diff := now.Sub(creationTime).Minutes()
	//Timeout 30 minutes from creation time
	if diff >= 30 {
		cluster.Status.SetTypedPhase(clusterv1.ClusterPhaseFailed)
		cloud.HandleUpdateVCError(ViettelCluster, errors.Errorf("Timeout provisioning cluster"))
		return errors.Errorf("Timeout provisioning cluster")
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ViettelClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ViettelCluster{}).
		Complete(r)
}
