package servers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/codius/codius-crd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServicesApi struct {
	BindAddress string
	client.Client
	Log logr.Logger
}

func (api *ServicesApi) getService() httprouter.Handle {
	return func(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		ctx := context.Background()
		var codiusService v1alpha1.Service
		if err := api.Get(ctx, types.NamespacedName{Name: ps.ByName("name"), Namespace: ""}, &codiusService); err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Header().Set("Content-Type", "application/json; charset=UTF-8")
		rw.WriteHeader(http.StatusOK)
		// Exclude secretData and internal fields
		if err := json.NewEncoder(rw).Encode(&v1alpha1.Service{
			TypeMeta: codiusService.TypeMeta,

			ObjectMeta: metav1.ObjectMeta{
				Name: codiusService.ObjectMeta.Name,
				// Empty creationTimestamp currently isn't omitted
				// https://github.com/kubernetes/kubernetes/issues/67610
				CreationTimestamp: codiusService.ObjectMeta.CreationTimestamp,
				Annotations: map[string]string{
					"codius.org/spec-hash": codiusService.ObjectMeta.Annotations["codius.org/spec-hash"],
					"codius.org/hostname":  codiusService.ObjectMeta.Annotations["codius.org/hostname"],
				},
			},
			Spec: codiusService.Spec,
		}); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
		}
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
	srv := &http.Server{
		Addr:    api.BindAddress,
		Handler: router,
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
