/*
Copyright 2018 The Kubernetes Authors.

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

package webhooks

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/hash"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	validAnnotation = regexp.MustCompile(`^[a-f0-9]{64}$`)
	suffixLen       = 11
)

// +kubebuilder:webhook:path=/validate-v1-secret,mutating=false,failurePolicy=fail,groups="",resources=secrets,verbs=create;update,versions=v1,name=vsecret.kb.io

// SecretValidator validates Secrets
type SecretValidator struct {
	decoder *admission.Decoder
}

// SecretValidator admits a secret iff its name is suffixed with the sha256 of its data.
func (v *SecretValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}
	err := v.decoder.Decode(req, secret)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		return admission.Denied(fmt.Sprintf("type must be %s", corev1.SecretTypeOpaque))
	}
	key := "codius.hash"
	anno, found := secret.Annotations[key]
	if !found {
		return admission.Denied(fmt.Sprintf("missing annotation %s", key))
	}
	if !validAnnotation.MatchString(anno) {
		return admission.Denied(fmt.Sprintf("annotation %s must match %s", key, validAnnotation.String()))
	}
	if len(secret.Name) <= suffixLen {
		return admission.Denied(fmt.Sprintf("name must include hash suffix"))
	}
	secretNoSuffix := *secret
	secretNoSuffix.Name = secret.Name[:len(secret.Name)-suffixLen]
	h, err := hash.SecretHash(&secretNoSuffix)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if secret.Name != fmt.Sprintf("%s-%s", secretNoSuffix.Name, h) {
		return admission.Denied(fmt.Sprintf("name must include hash suffix"))
	}
	return admission.Allowed("")
}

// SecretValidator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *SecretValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
