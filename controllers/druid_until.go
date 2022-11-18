package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
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
