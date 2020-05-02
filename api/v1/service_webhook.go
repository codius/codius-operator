/*


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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var c client.Client

// log is for logging in this package.
var servicelog = logf.Log.WithName("service-resource")

func (r *Service) SetupWebhookWithManager(mgr ctrl.Manager) error {
	c = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-core-codius-org-v1-service,mutating=true,failurePolicy=fail,groups=core.codius.org,resources=services,verbs=create;update,versions=v1,name=mservice.kb.io

var _ webhook.Defaulter = &Service{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Service) Default() {
	servicelog.Info("default", "service", r)
	servicelog.Info("default", "name", r.Name)
	hash, err := getSha256(&r.Spec)
	if err != nil {
		return
	}
	r.Name = hash
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-core-codius-org-v1-service,mutating=false,failurePolicy=fail,groups=core.codius.org,resources=services,versions=v1,name=vservice.kb.io

var _ webhook.Validator = &Service{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateCreate() error {
	servicelog.Info("validate create", "name", r.Name)

	if err := r.ValidateName(); err != nil {
		return err
	}
	if err := r.ValidateEnvSources(); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateUpdate(old runtime.Object) error {
	servicelog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateDelete() error {
	servicelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Service) ValidateEnvSources() error {
	// Verify all env secrets belong to this service
	ctx := context.Background()
	var secrets map[string]bool
	for i, container := range r.Spec.Containers {
		for j, env := range container.Env {
			if env.ValueFrom == nil || secrets[env.ValueFrom.SecretKeyRef.LocalObjectReference.Name] == false {
				var secret corev1.Secret
				if err := c.Get(ctx, types.NamespacedName{Name: env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, Namespace: r.Namespace}, &secret); err != nil {
					return err
				}
				// check that secret has annotation with service name
				if secret.Annotations["codius.service"] != r.Name {
					return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
						field.Invalid(field.NewPath("spec").Child("containers").Index(i).Child("env").Index(j).Child("valueFrom").Child("secretKeyRef").Child("localObjectReference").Child("name"), env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, "env secret must have matching \"codius.service\" annotation"),
					})
				}

				// Can this instead assume secret validating webhook guarantees secret name?
				// hash, err := getSha256(&secret.Data)
				// if err != nil {
				// 	return err
				// }
				// if hash != secret.Name {
				// 	return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
				// 		field.Invalid(field.NewPath("spec").Child("envFrom").Child("secretRef").Child("hash"), envFrom.SecretRef.Hash, "envFrom hash must match sha256 of secret data"),
				// 	})
				// }

				secrets[env.ValueFrom.SecretKeyRef.LocalObjectReference.Name] = true
			}
		}
	}

	return nil
}

func (r *Service) ValidateName() error {
	hash, err := getSha256(&r.Spec)
	if err != nil {
		return err
	}
	if hash != r.Name {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "name must be sha256 of spec"),
		})
	}
	return nil
}

func getSha256(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}
