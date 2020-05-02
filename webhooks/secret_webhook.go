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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-v1-secret,mutating=false,failurePolicy=fail,groups="",resources=secrets,verbs=create;update,versions=v1,name=vsecret.kb.io

// SecretValidator validates Secrets
type SecretValidator struct {
	// Client  client.Client
	decoder *admission.Decoder
}

// SecretValidator admits a secret iff its name is the sha256 of its data.
func (v *SecretValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}

	err := v.decoder.Decode(req, secret)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	key := "codius.service"
	// anno, found := secret.Annotations[key]
	_, found := secret.Annotations[key]
	if !found {
		return admission.Denied(fmt.Sprintf("missing annotation %s", key))
	}
	// check annotation regex
	// if anno != "foo" {
	// 	return admission.Denied(fmt.Sprintf("annotation %s did not have value %q", key, "foo"))
	// }
	hash, err := getSha256(&secret.Data)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if hash != secret.Name {
		return admission.Denied(fmt.Sprintf("name must match sha256 of data"))
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

func getSha256(data *map[string][]byte) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}
