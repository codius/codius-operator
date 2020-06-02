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

package v1alpha1

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

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

// +kubebuilder:webhook:path=/mutate-core-codius-org-v1alpha1-service,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.codius.org,resources=services,verbs=create;update,versions=v1alpha1,name=mservice.kb.io

var _ webhook.Defaulter = &Service{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Service) Default() {
	servicelog.Info("default", "service", r)
	servicelog.Info("default", "name", r.Name)
	hash, err := r.getSha256()
	if err != nil {
		return
	}

	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations["codius.hash"] = hash

	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	// a DNS-1035 label must consist of lower case alphanumeric characters or '-',
	// start with an alphabetic character, and end with an alphanumeric character
	// (e.g. 'my-name',  or 'abc-123', regex used for validation is '[a-z]([-a-z0-9]*[a-z0-9])?'
	r.Labels["app"] = "codius-" + (hash)[:56]
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-core-codius-org-v1alpha1-service,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.codius.org,resources=services,versions=v1alpha1,name=vservice.kb.io

var _ webhook.Validator = &Service{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateCreate() error {
	servicelog.Info("validate create", "name", r.Name)

	if err := r.ValidateHash(); err != nil {
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
	secrets := map[string]bool{}
	for i, container := range r.Spec.Containers {
		for j, env := range container.Env {
			if env.ValueFrom != nil && secrets[env.ValueFrom.SecretKeyRef.LocalObjectReference.Name] == false {
				var secret corev1.Secret
				if err := c.Get(ctx, types.NamespacedName{Name: env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, Namespace: r.Namespace}, &secret); err == nil {
					// check that secret has annotation with service name
					// it's ok if the secret doesn't exist yet
					if secret.Annotations["codius.hash"] != r.Annotations["codius.hash"] {
						return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
							field.Invalid(field.NewPath("spec").Child("containers").Index(i).Child("env").Index(j).Child("valueFrom").Child("secretKeyRef").Child("localObjectReference").Child("name"), env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, "env secret must have matching \"codius.hash\" annotation"),
						})
					}
				}
				secrets[env.ValueFrom.SecretKeyRef.LocalObjectReference.Name] = true
			}
		}
	}

	return nil
}

func (r *Service) ValidateHash() error {
	hash, err := r.getSha256()
	if err != nil {
		return err
	}
	if r.Annotations["codius.hash"] != hash {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("annotations").Child("codius.hash"), r.Name, "codius.hash annotation must be sha256 of spec"),
		})
	}
	if r.Labels["app"] != "codius-"+(hash)[:56] {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("annotations").Child("codius.hash"), r.Name, "app label must have sha256 of spec"),
		})
	}
	return nil
}

func (r *Service) getSha256() (string, error) {
	data, err := json.Marshal(r.Spec)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data))), nil
}
