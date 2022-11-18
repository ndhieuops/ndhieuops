package controllers

import (
	"context"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

func getOwner(ncMeta metav1.ObjectMeta) metav1.OwnerReference {
	owners := ncMeta.GetOwnerReferences()
	if len(owners) == 0 {
		return metav1.OwnerReference{}
	}
	for _, owner := range owners {
		if owner.APIVersion == controlplanev1.GroupVersion.String() &&
			owner.Kind == "NestedControlPlane" {
			return owner
		}
	}
	return metav1.OwnerReference{}
}
func IsComponentReady(status addonv1alpha1.CommonStatus) bool {
	return status.Phase == string(controlplanev1.Ready)
}
func createNestedEtcd(ctx context.Context,
	cli ctrlcli.Client, ncMeta metav1.ObjectMeta,
	ncSpec controlplanev1.NestedComponentSpec,
	ncKind, clusterName string, log logr.Logger) error {
	// set up the ownerReferences for all objects
	or := metav1.NewControllerRef(&ncMeta,
		controlplanev1.GroupVersion.WithKind(ncKind))
	etcd, err := genEtcdDruidObject(cli, ncMeta, ncSpec, ncKind, clusterName, log)
	if err != nil {
		return errors.Errorf("fail to generate the Etcd-Druid object: %v", err)
	}
	controller := "controller-manager"
	if ncKind != controller {
		// no need to create the service for the NestedControllerManager
		ncSvc, err := genServiceObject(ncKind, clusterName, ncMeta.GetName(), ncMeta.GetNamespace())
		if err != nil {
			return errors.Errorf("fail to generate the Service object: %v", err)
		}

		ncSvc.SetOwnerReferences([]metav1.OwnerReference{*or})
		if err := cli.Create(ctx, ncSvc); err != nil {
			if err != nil {
				log.Info(err.Error())
			}
			return err
		}
		log.Info("successfully create the service for the Etcd-Druid",
			"component", ncKind)
	}
	// set the NestedComponent object as the owner of the Etcd Object
	etcd.SetOwnerReferences([]metav1.OwnerReference{*or})
	// create the Etcd resources
	return cli.Create(ctx, etcd)
}

func genEtcdDruidObject(
	cli ctrlcli.Client,
	ncMeta metav1.ObjectMeta,
	ncSpec controlplanev1.NestedComponentSpec,
	ncKind, clusterName string,
	log logr.Logger) (*druidv1alpha1.Etcd, error) {
	// 2. generate the etcd manifest
	ems, err := genEtcdManifest(ncKind, clusterName, ncMeta.GetName(), ncMeta.GetNamespace())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate the Etcd-Druid manifest")
	}
	// 3. Update values of the Etcd-Druid to NestedEtcd
	if ncSpec.Replicas != 0 {
		ems.Spec.Replicas = int(ncSpec.Replicas)
	}
	log.V(5).Info("The Etcd-Druid Resources are set ",
		"Etcd-Druid", ems.GetName())
	return ems, nil
}
func genEtcdManifest(ncKind, clusterName, componentName, componentNamespace string) (*druidv1alpha1.Etcd, error) {
	etcdtype := "etcd"
	switch ncKind {
	case etcdtype:
	default:
		return nil, errors.Errorf("invalid component type: %s", ncKind)
	}
	return &druidv1alpha1.Etcd{
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterName + "-" + ncKind,
			Namespace:       componentNamespace,
			OwnerReferences: []metav1.OwnerReference{},
			Finalizers:      []string{},
		},
		Spec: druidv1alpha1.EtcdSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": componentName,
				},
			},
			Labels: map[string]string{
				"app": componentName,
			},
			Etcd: druidv1alpha1.EtcdConfig{
				TLS: &druidv1alpha1.TLSConfig{
					ClientTLSSecretRef: corev1.SecretReference{
						Name:      "cluster-sample-etcd-client",
						Namespace: "default",
					},
					TLSCASecretRef: corev1.SecretReference{
						Name:      "cluster-sample-etcd",
						Namespace: "default",
					},
				},
			},
			Backup:       druidv1alpha1.BackupSpec{},
			Replicas:     3,
			StorageClass: new(string),
		},
		Status: druidv1alpha1.EtcdStatus{},
	}, nil
}

func genServiceObject(ncKind, clusterName, componentName, componentNamespace string) (*corev1.Service, error) {
	etcdtype := "etcd"
	switch ncKind {
	case etcdtype:
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName + "-etcd",
				Namespace: componentNamespace,
				Labels: map[string]string{
					"component-name": componentName,
				},
			},
			Spec: corev1.ServiceSpec{
				PublishNotReadyAddresses: true,
				ClusterIP:                "None",
				Selector: map[string]string{
					"component-name": componentName,
				},
			},
		}, nil
	default:
		return nil, errors.Errorf("unknown component type: %s", ncKind)
	}
}
