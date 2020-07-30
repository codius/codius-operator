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
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var c client.Client

// log is for logging in this package.
var servicelog = logf.Log.WithName("service-resource")

var validHash = regexp.MustCompile(`^[a-z2-8]{52}$`)

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

	if r.SecretData != nil {
		secretHash, err := r.hashSecret()
		if err != nil {
			return
		}
		for _, container := range r.Spec.Containers {
			for _, env := range container.Env {
				if env.ValueFrom != nil {
					env.ValueFrom.SecretKeyRef.Hash = secretHash
				}
			}
		}
	}

	hash, err := r.hashSpec()
	if err != nil {
		return
	}

	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	r.Annotations["codius.org/hash"] = hash
	r.Annotations["codius.org/hostname"] = fmt.Sprintf("%s.%s", r.Name, os.Getenv("CODIUS_HOSTNAME"))

	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	// a DNS-1035 label must consist of lower case alphanumeric characters or '-',
	// start with an alphabetic character, and end with an alphanumeric character
	// (e.g. 'my-name',  or 'abc-123', regex used for validation is '[a-z]([-a-z0-9]*[a-z0-9])?'
	// and must be no more than 63 characters.
	r.Labels["codius.org/service"] = "svc-" + hash
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-core-codius-org-v1alpha1-service,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.codius.org,resources=services,versions=v1alpha1,name=vservice.kb.io

var _ webhook.Validator = &Service{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateCreate() error {
	servicelog.Info("validate create", "name", r.Name)

	return r.ValidateService()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateUpdate(old runtime.Object) error {
	servicelog.Info("validate update", "name", r.Name)

	if r.Labels["codius.org/token"] != old.(*Service).Labels["codius.org/token"] {
		return errors.NewForbidden(schema.GroupResource{Group: "core.codius.org", Resource: r.Kind}, r.Name,
			field.Invalid(field.NewPath("metadata").Child("labels").Child("codius.org/token"), r.Labels["codius.org/token"], "codius.org/token label must match existing resource"))
	}

	return r.ValidateService()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Service) ValidateDelete() error {
	servicelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Service) ValidateService() error {
	if err := r.ValidateHash(); err != nil {
		return err
	}
	if err := r.ValidateName(); err != nil {
		return err
	}
	if err := r.ValidateSecretData(); err != nil {
		return err
	}
	return nil
}

func (r *Service) ValidateSecretData() error {
	var secretHash string
	if r.SecretData != nil {
		var err error
		secretHash, err = r.hashSecret()
		if err != nil {
			return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
				field.Invalid(field.NewPath("secretData"), r.SecretData, "unable to hash secretData"),
			})
		}
	}
	for i, container := range r.Spec.Containers {
		for j, env := range container.Env {
			if env.ValueFrom != nil {
				if env.Value != "" {
					return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
						field.Invalid(field.NewPath("spec").Child("containers").Index(i).Child("env").Index(j), env, "env value and valueFrom are mutually exclusive"),
					})
				}
				if _, ok := r.SecretData[env.ValueFrom.SecretKeyRef.Key]; !ok {
					return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
						field.Invalid(field.NewPath("spec").Child("containers").Index(i).Child("env").Index(j).Child("valueFrom").Child("secretKeyRef").Child("key"), env.ValueFrom.SecretKeyRef.Key, "missing env secret data"),
					})
				} else if env.ValueFrom.SecretKeyRef.Hash != secretHash {
					return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
						field.Invalid(field.NewPath("spec").Child("containers").Index(i).Child("env").Index(j).Child("valueFrom").Child("secretKeyRef").Child("hash"), env.ValueFrom.SecretKeyRef.Hash, "invalid env secret hash"),
					})
				}
			}
		}
	}

	return nil
}

func (r *Service) ValidateHash() error {
	hash, err := r.hashSpec()
	if err != nil {
		return err
	}
	if r.Annotations["codius.org/hash"] != hash {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("annotations").Child("codius.org/hash"), r.Annotations["codius.org/hash"], "codius.org/hash annotation must be sha256 of spec"),
		})
	}
	if r.Labels["codius.org/service"] != "svc-"+hash {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("labels").Child("codius.org/service"), r.Labels["codius.org/service"], "codius.org/service label must have sha256 of spec"),
		})
	}
	return nil
}

func (r *Service) ValidateName() error {
	if r.Labels["codius.org/immutable"] == "true" {
		if r.Annotations["codius.org/hash"] != r.Name {
			return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
				field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "name must be sha256 of spec"),
			})
		}
	} else if validHash.MatchString(r.Name) {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "name must NOT be a sha256 hash"),
		})
	}
	return nil
}

func (r *Service) hashSpec() (string, error) {
	data, err := json.Marshal(r.Spec)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(data))
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])), nil
}

func (r *Service) hashSecret() (string, error) {
	data, err := json.Marshal(r.SecretData)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(data))
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:])), nil
}
