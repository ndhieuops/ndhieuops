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

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/certificate"
	controlplanev1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DruidReconciler reconciles a Druid object
type DruidReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=etcd.druid.cloud.etcd.druid.cloud,resources=druids,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=etcd.druid.cloud.etcd.druid.cloud,resources=druids/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=etcd.druid.cloud.etcd.druid.cloud,resources=druids/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Druid object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *DruidReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Etcd", req.NamespacedName)
	log.Info("Reconciling Etcd...")
	var netcd controlplanev1.NestedEtcd
	if err := r.Get(ctx, req.NamespacedName, &netcd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("creating Etcd",
		"namespace", netcd.GetNamespace(),
		"name", netcd.GetName())

	// check if the ownerreference has been set by the NestedControlPlane controller.
	owner := getOwner(netcd.ObjectMeta)
	if owner == (metav1.OwnerReference{}) {
		// requeue the request if the owner NestedControlPlane has
		// not been set yet.
		log.Info("the owner has not been set yet, will retry later",
			"namespace", netcd.GetNamespace(),
			"name", netcd.GetName())
		return ctrl.Result{Requeue: true}, nil
	}

	var ncp controlplanev1.NestedControlPlane
	if err := r.Get(ctx, types.NamespacedName{Namespace: netcd.GetNamespace(), Name: owner.Name}, &ncp); err != nil {
		log.Info("the owner could not be found, will retry later",
			"namespace", netcd.GetNamespace(),
			"name", owner.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster, err := ncp.GetOwnerCluster(ctx, r.Client)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the control plane")
		return ctrl.Result{}, err
	}
	etcdtype := "etcd"
	etcdName := fmt.Sprintf("%s-etcd", cluster.GetName())
	var etcddruid druidv1alpha1.Etcd
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      etcdName,
	}, &etcddruid); err != nil {
		if apierrors.IsNotFound(err) {
			// as the EtcdDruid is not found, mark the Etcd as unready
			if IsComponentReady(netcd.Status.CommonStatus) {
				netcd.Status.Phase =
					string(controlplanev1.Unready)
				log.V(5).Info("The corresponding Etcd-Druid is not found, " +
					"will mark the Etcd as unready")
				if err := r.Status().Update(ctx, &netcd); err != nil {
					log.Error(err, "fail to update the status of the Etcd Object")
					return ctrl.Result{}, err
				}
			}
			if err := r.createEtcdClientCrts(ctx, cluster, &ncp, &netcd); err != nil {
				log.Error(err, "fail to create Etcd Client Certs")
				return ctrl.Result{}, err
			}

			// the statefulset is not found, create one
			if err := createNestedEtcd(ctx,
				r.Client, netcd.ObjectMeta,
				netcd.Spec.NestedComponentSpec,
				etcdtype, cluster.GetName(), log); err != nil {
				log.Error(err, "fail to create Etcd Object1")
				return ctrl.Result{}, err
			}

			log.Info("successfully create the Etcd Object")
			return ctrl.Result{}, nil
		}
		log.Error(err, "|    fail to get Etcd Object    |")
		return ctrl.Result{}, err
	}

	if etcddruid.Status.ReadyReplicas == etcddruid.Status.Replicas {
		log.Info("The Etcd is ready")
		if !IsComponentReady(netcd.Status.CommonStatus) {
			// As the Etcd StatefulSet is ready, update Etcd status
			ip, err := getEtcdSvcClusterIP(ctx, r.Client, cluster.GetName(), &netcd)
			if err != nil {
				log.Error(err, "fail to get Etcd Service ClusterIP")
				return ctrl.Result{}, err
			}
			netcd.Status.Phase = string(controlplanev1.Ready)
			netcd.Status.Addresses = []controlplanev1.NestedEtcdAddress{
				{
					IP:   ip,
					Port: 2379,
				},
			}
			log.V(5).Info("The Etcd is ready")
			if err := r.Status().Update(ctx, &netcd); err != nil {
				log.Error(err, "fail to update Etcd Object")
				return ctrl.Result{}, err
			}
			log.Info("Successfully set the Etcd object to ready",
				"address", netcd.Status.Addresses)
		}
		return ctrl.Result{}, nil
	}

	// As the Etcd StatefulSet is unready, mark the Etcd as unready
	// if its current status is ready
	if IsComponentReady(netcd.Status.CommonStatus) {
		netcd.Status.Phase = string(controlplanev1.Unready)
		if err := r.Status().Update(ctx, &netcd); err != nil {
			log.Error(err, "fail to update Etcd Object slat")
			return ctrl.Result{}, err
		}
		log.Info("Successfully set the Etcd object to unready")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DruidReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(),
		&druidv1alpha1.Etcd{},
		statefulsetOwnerKeyNEtcd,
		func(rawObj client.Object) []string {
			// grab the statefulset object, extract the owner
			sts := rawObj.(*druidv1alpha1.Etcd)
			owner := metav1.GetControllerOf(sts)
			if owner == nil {
				return nil
			}
			// make sure it's a Etcd
			if owner.APIVersion != controlplanev1.GroupVersion.String() ||
				owner.Kind != "Etcd" {
				return nil
			}

			// and if so, return it
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.NestedEtcd{}).
		Owns(&druidv1alpha1.Etcd{}).
		Complete(r)
}
func getEtcdSvcClusterIP(ctx context.Context, cli client.Client,
	clusterName string, netcd *controlplanev1.NestedEtcd) (string, error) {
	var svc corev1.Service
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      fmt.Sprintf("%s-etcd", clusterName),
	}, &svc); err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func getEtcdServers(name, namespace string, replicas int32) (etcdServers []string) {
	var i int32
	for ; i < replicas; i++ {
		etcdServers = append(etcdServers, fmt.Sprintf("%s-etcd-%d.%s-etcd.%s", name, i, name, namespace))
	}
	etcdServers = append(etcdServers, name)
	return etcdServers
}

// createEtcdClientCrts will find of create client certs for the etcd cluster.
func (r *DruidReconciler) createEtcdClientCrts(ctx context.Context, cluster *controlplanev1alpha4.Cluster, ncp *controlplanev1.NestedControlPlane, netcd *controlplanev1.NestedEtcd) error {
	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	if err := certificates.Lookup(ctx, r.Client, util.ObjectKey(cluster)); err != nil {
		fmt.Println(err)
		return err
	}

	cert := certificates.GetByPurpose(secret.EtcdCA)
	if cert == nil {
		return fmt.Errorf("could not fetch EtcdCA")
	}

	crt, err := certs.DecodeCertPEM(cert.KeyPair.Cert)
	if err != nil {
		return err
	}

	key, err := certs.DecodePrivateKeyPEM(cert.KeyPair.Key)
	if err != nil {
		return err
	}

	etcdKeyPair, err := certificate.NewEtcdServerCertAndKey(&certificate.KeyPair{Cert: crt, Key: key}, getEtcdServers(cluster.GetName(), cluster.GetNamespace(), netcd.Spec.Replicas))
	if err != nil {
		return err
	}

	certs := &certificate.KeyPairs{
		etcdKeyPair,
	}

	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	return certs.LookupOrSave(ctx, r.Client, util.ObjectKey(cluster), *controllerRef)
}
