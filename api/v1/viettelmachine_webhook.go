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

package v1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const ViettelMachineImmutableMsg = "ViettelMachine spec field is immutable. Please create a new resource instead."

var ViettelMachineLog = logf.Log.WithName("ViettelMachine-resource")

func (r *ViettelMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-git-viettel-vn-v1-ViettelMachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.git.viettel.vn,resources=ViettelMachines,verbs=create;update,versions=v1,name=ViettelMachine.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ViettelMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ViettelMachine) ValidateCreate() error {
	ViettelMachineLog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ViettelMachine) ValidateUpdate(old runtime.Object) error {
	ViettelMachineLog.Info("validate update", "name", r.Name)
	var allErrs field.ErrorList
	oldNestedcluster := old.(*ViettelMachine)

	if !reflect.DeepEqual(r.Spec, oldNestedcluster.Spec) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "template", "spec"), r, ViettelMachineImmutableMsg),
		)
	}

	if len(allErrs) != 0 {
		return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ViettelMachine) ValidateDelete() error {
	ViettelMachineLog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
