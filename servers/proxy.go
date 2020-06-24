package servers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/codius/codius-crd-operator/api/v1alpha1"
	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Proxy struct {
	BindAddress string
	client.Client
	Log logr.Logger
}

func (proxy *Proxy) deductBalance(id *string) error {
	url := fmt.Sprintf("%s/balances/%s:spend", os.Getenv("RECEIPT_VERIFIER_URL"), *id)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer([]byte(os.Getenv("REQUEST_PRICE"))))
	if err != nil {
		proxy.Log.Error(err, "Failed to spend balance")
		return err
	}
	b, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		err = errors.New(string(b))
		proxy.Log.Error(err, "Failed to spend balance")
		return err
	}
	return nil
}

func (proxy *Proxy) Start(stopCh <-chan struct{}) error {
	svr := proxy.start()
	defer proxy.stop(svr)

	<-stopCh
	return nil
}

func (proxy *Proxy) start() *http.Server {
	http.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		proxy.Log.Info(req.Host)
		serviceName := strings.SplitN(req.Host, ".", 2)[0]
		ctx := context.Background()
		var codiusService v1alpha1.Service
		if err := proxy.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: ""}, &codiusService); err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		var proxyUrl string
		if codiusService.Status.UnavailableReplicas > int32(0) && codiusService.Status.AvailableReplicas == int32(0) {
			proxyUrl = fmt.Sprintf("%s/%s/503", os.Getenv("CODIUS_WEB_URL"), serviceName)
		} else {
			err := proxy.deductBalance(&serviceName)
			if err != nil {
				proxyUrl = fmt.Sprintf("%s/%s/402", os.Getenv("CODIUS_WEB_URL"), serviceName)
			} else {
				codiusService.Status.LastRequestTime = &metav1.Time{Time: time.Now()}
				if err := proxy.Status().Update(ctx, &codiusService); err != nil {
					proxy.Log.Error(err, "unable to update LastRequestTime")
				}
				if codiusService.Status.AvailableReplicas > int32(0) {
					proxyUrl = fmt.Sprintf("http://%s.%s", codiusService.Labels["app"], os.Getenv("CODIUS_NAMESPACE"))
				} else {
					proxyUrl = fmt.Sprintf("%s/%s/503", os.Getenv("CODIUS_WEB_URL"), serviceName)
				}
			}
		}
		url, _ := url.Parse(proxyUrl)
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(rw, req)
	})
	srv := &http.Server{
		Addr: proxy.BindAddress,
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			proxy.Log.Error(err, "Failed to run http server")
		}
	}()
	return srv
}

func (proxy *Proxy) stop(srv *http.Server) {
	if err := srv.Shutdown(nil); err != nil {
		proxy.Log.Error(err, "Error shutting down http server")
	}
}
