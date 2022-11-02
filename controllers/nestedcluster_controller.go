/*
Copyright 2021 The Kubernetes Authors.

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

// Package controllers contains all the Infrastructure group controllers for
// running nested clusters.
package controllers

import (
	"context"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	infrav1 "sigs.k8s.io/cluster-api-provider-nested/api/v1alpha4"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
)

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nestedclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nestedclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nestedclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes,verbs=get;list;watch

// NestedClusterReconciler reconciles a NestedCluster object.
type NestedClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *NestedClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	clusterToInfraFn := util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("NestedCluster"))
	log := ctrl.LoggerFrom(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.NestedCluster{},
			builder.WithPredicates(
				predicate.Funcs{
					// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
					UpdateFunc: func(e event.UpdateEvent) bool {
						oldCluster := e.ObjectOld.(*infrav1.NestedCluster).DeepCopy()
						newCluster := e.ObjectNew.(*infrav1.NestedCluster).DeepCopy()
						oldCluster.Status = infrav1.NestedClusterStatus{}
						newCluster.Status = infrav1.NestedClusterStatus{}
						oldCluster.ObjectMeta.ResourceVersion = ""
						newCluster.ObjectMeta.ResourceVersion = ""
						return !reflect.DeepEqual(oldCluster, newCluster)
					},
				},
			),
		).
		Owns(&controlplanev1.NestedControlPlane{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				requests := clusterToInfraFn(o)
				if len(requests) < 1 {
					return nil
				}

				c := &infrav1.NestedCluster{}
				if err := r.Client.Get(ctx, requests[0].NamespacedName, c); err != nil {
					log.V(4).Error(err, "Failed to get Nested cluster")
					return nil
				}

				if annotations.IsExternallyManaged(c) {
					log.V(4).Info("Nested cluster is externally managed, skipping mapping.")
					return nil
				}
				return requests
			}),
			builder.WithPredicates(predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx))),
		).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(ctrl.LoggerFrom(ctx))).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NestedCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *NestedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedcluster", req.NamespacedName)
	log.Info("Reconciling NestedCluster...")
	nc := &infrav1.NestedCluster{}
	if err := r.Get(ctx, req.NamespacedName, nc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, nc.ObjectMeta)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the control plane")
		return ctrl.Result{}, err
	}

	objectKey := types.NamespacedName{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	ncp := &controlplanev1.NestedControlPlane{}
	if err := r.Get(ctx, objectKey, ncp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if !nc.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, req, cluster, nc)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, req, cluster, nc)

	if !nc.Status.Ready && ncp.Status.Ready && ncp.Status.Initialized {
		nc.Status.Ready = true
		if err := r.Status().Update(ctx, nc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *NestedClusterReconciler) reconcileDelete(ctx context.Context, req ctrl.Request, cluster *clusterv1.Cluster, nc *infrav1.NestedCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx, "cluster", cluster.Name)
	log.Info("Reconcile KubeadmControlPlane deletion")
	log.Info(nc.Name)
	return ctrl.Result{}, nil
}

func (r *NestedClusterReconciler) reconcileNormal(ctx context.Context, req ctrl.Request, cluster *clusterv1.Cluster, nc *infrav1.NestedCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx, "cluster", cluster.Name)
	log.Info("Reconcile KubeadmControlPlane normal")
	log.Info(nc.Name)
	//get etcd cluster, create if not found   ---> check ETCD Cluster (druin)
	etcd := &druidv1alpha1.Etcd{}

	if err := r.Client.Get(ctx, req.NamespacedName, etcd); err != nil {
		if apierrors.IsNotFound(err) {
			etcd.Name = nc.Name
			etcd.Namespace = nc.Namespace
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
			ds.Name = nc.Name
			ds.Namespace = nc.Namespace
			ds.Spec = kamajiv1alpha1.DataStoreSpec{}
			if err := r.Client.Create(ctx, ds); err != nil {
				log.Info("Fail to create data store")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	//get Tenant Control Plane , create if not found   --> check Tenant Control Plane (Kamaji)
	tcp := &kamajiv1alpha1.TenantControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, tcp); err != nil {
		if apierrors.IsNotFound(err) {
			tcp.Name = nc.Name
			tcp.Namespace = nc.Namespace
			tcp.Spec = kamajiv1alpha1.TenantControlPlaneSpec{}
			if err := r.Client.Create(ctx, tcp); err != nil {
				log.Info("Fail to create data store")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}
	// Set Control Plane Endpoint for Control Plane
	// Set Control Plane Endpoint for Infra Provider
	if tcp.Status.ControlPlaneEndpoint != "" {
		nc.Spec.ControlPlaneEndpoint.Host = ""
		nc.Spec.ControlPlaneEndpoint.Port = 0
		nc.Status.Ready = true
		nc.Status.
		//update nc

	}

	// Set Status Provision and Ready

	return ctrl.Result{}, nil
}
