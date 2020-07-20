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

package servers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/codius/codius-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServicesApi struct {
	BindAddress string
	client.Client
	Log logr.Logger
}

type Service struct {
	Spec       v1alpha1.ServiceSpec
	SecretData map[string]string
}

func (api *ServicesApi) createService() httprouter.Handle {
	return func(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}
		authHeaderParts := strings.Fields(authHeader)
		if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
			http.Error(rw, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
			return
		}
		token := authHeaderParts[1]
		name := ps.ByName("name")
		var service Service
		dec := json.NewDecoder(req.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&service); err != nil {
			api.Log.Error(err, "Failed to decode service", "Service.Name", name)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := deductBalance(&token, os.Getenv("SERVICE_PRICE")); err != nil {
			api.Log.Error(err, "Failed to spend balance", "Service.Name", name)
			rw.WriteHeader(http.StatusPaymentRequired)
			return
		}
		ctx := req.Context()
		codiusService := v1alpha1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec:       service.Spec,
			SecretData: service.SecretData,
		}
		if err := api.Create(ctx, &codiusService); err != nil {
			api.Log.Error(err, "Failed to create new Service.", "Service.Name", name)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusCreated)
	}
}

func (api *ServicesApi) getService() httprouter.Handle {
	return func(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		ctx := req.Context()
		var codiusService v1alpha1.Service
		if err := api.Get(ctx, types.NamespacedName{Name: ps.ByName("name"), Namespace: ""}, &codiusService); err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		data, err := json.Marshal(codiusService.Sanitize())
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json; charset=UTF-8")
		rw.WriteHeader(http.StatusOK)
		rw.Write(data)
	}
}

func (api *ServicesApi) Start(stopCh <-chan struct{}) error {
	svr := api.start()
	defer api.stop(svr)

	<-stopCh
	return nil
}

func (api *ServicesApi) start() *http.Server {
	router := httprouter.New()
	router.GET("/services/:name", api.getService())
	router.PUT("/services/:name", api.createService())
	srv := &http.Server{
		Addr:    api.BindAddress,
		Handler: cors.Default().Handler(router),
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			api.Log.Error(err, "Failed to run http server")
		}
	}()
	return srv
}

func (api *ServicesApi) stop(srv *http.Server) {
	if err := srv.Shutdown(nil); err != nil {
		api.Log.Error(err, "Error shutting down http server")
	}
}
