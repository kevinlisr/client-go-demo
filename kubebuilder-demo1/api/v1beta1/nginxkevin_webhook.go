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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var nginxkevinlog = logf.Log.WithName("nginxkevin-resource")

func (r *Nginxkevin) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-kevinapp-kevin-com-v1beta1-nginxkevin,mutating=true,failurePolicy=fail,sideEffects=None,groups=kevinapp.kevin.com,resources=nginxkevins,verbs=create;update,versions=v1beta1,name=mnginxkevin.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Nginxkevin{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Nginxkevin) Default() {
	nginxkevinlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-kevinapp-kevin-com-v1beta1-nginxkevin,mutating=false,failurePolicy=fail,sideEffects=None,groups=kevinapp.kevin.com,resources=nginxkevins,verbs=create;update,versions=v1beta1,name=vnginxkevin.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Nginxkevin{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Nginxkevin) ValidateCreate() error {
	nginxkevinlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Nginxkevin) ValidateUpdate(old runtime.Object) error {
	nginxkevinlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Nginxkevin) ValidateDelete() error {
	nginxkevinlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
