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
	"encoding/json"
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var secretlog = logf.Log.WithName("secret-resource")
var validHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (r *Secret) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-core-codius-org-v1alpha1-secret,mutating=true,failurePolicy=fail,groups=core.codius.org,resources=secrets,verbs=create;update,versions=v1alpha1,name=msecret.kb.io

var _ webhook.Defaulter = &Secret{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Secret) Default() {
	servicelog.Info("default", "secret", r)
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
	r.Labels["app"] = "codius-" + (r.ServiceHash)[:56]
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-core-codius-org-v1alpha1-secret,mutating=false,failurePolicy=fail,groups=core.codius.org,resources=secrets,versions=v1alpha1,name=vsecret.kb.io

var _ webhook.Validator = &Secret{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Secret) ValidateCreate() error {
	secretlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Secret) ValidateUpdate(old runtime.Object) error {
	secretlog.Info("validate update", "name", r.Name)

	if r.Annotations["codius.hash"] != r.Name {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "name must be sha256 of data"),
		})
	}
	if !validHash.MatchString(r.ServiceHash) {
		return errors.NewInvalid(schema.GroupKind{Group: "core.codius.org", Kind: r.Kind}, r.Name, field.ErrorList{
			field.Invalid(field.NewPath("serviceHash"), r.Name, "serviceHash must have sha256 of service"),
		})
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Secret) ValidateDelete() error {
	secretlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Secret) getSha256() (string, error) {
	data, err := json.Marshal(r.Data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data))), nil
}
