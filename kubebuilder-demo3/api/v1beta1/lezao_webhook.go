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

package v1beta1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var lezaolog = logf.Log.WithName("lezao-resource")

func (r *Lezao) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-lecang-nancal-com-v1beta1-lezao,mutating=true,failurePolicy=fail,sideEffects=None,groups=lecang.nancal.com,resources=lezaoes,verbs=create;update,versions=v1beta1,name=mlezao.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Lezao{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Lezao) Default() {
	lezaolog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
	r.Spec.EnableIngress = !r.Spec.EnableIngress
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-lecang-nancal-com-v1beta1-lezao,mutating=false,failurePolicy=fail,sideEffects=None,groups=lecang.nancal.com,resources=lezaoes,verbs=create;update,versions=v1beta1,name=vlezao.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Lezao{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Lezao) ValidateCreate() error {
	lezaolog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return r.validateApp()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Lezao) ValidateUpdate(old runtime.Object) error {
	lezaolog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return r.validateApp()
}

func (r *Lezao) validateApp() error {
	if !r.Spec.EnableService && r.Spec.EnableIngress {
		return apierrors.NewInvalid(GroupVersion.WithKind("Lezao").GroupKind(), r.Name,
			field.ErrorList{
				field.Invalid(field.NewPath("enable_service"),
					r.Spec.EnableService,
					"enable_service should be true when enable_ingress is true"),
			},
		)
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Lezao) ValidateDelete() error {
	lezaolog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
