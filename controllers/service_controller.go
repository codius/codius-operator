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

package controllers

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/codius/codius-operator/api/v1alpha1"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.codius.org,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.codius.org,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:namespace=system,groups=apps,resources=deployments,verbs=list;watch;get;patch;create;update
// +kubebuilder:rbac:namespace=system,groups=core,resources=services,verbs=list;watch;get;patch;create;update

func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("service", req.NamespacedName)

	// your logic here
	var codiusService v1alpha1.Service
	if err := r.Get(ctx, req.NamespacedName, &codiusService); err != nil {
		log.Error(err, "unable to fetch Codius Service")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if codiusService.Labels["codius.org/immutable"] != "true" {
		// Check if the corresponding immutable service exists, if not create a new one
		var immutableService v1alpha1.Service
		err := r.Get(ctx, types.NamespacedName{Name: codiusService.Annotations["codius.org/hash"]}, &immutableService)
		if err != nil && errors.IsNotFound(err) {
			immutableService := codiusService.Immutify()
			// Do NOT set Codius Service as the owner and controller
			log.Info("Creating a new immutable Service", "Service.Name", immutableService.Name)
			err = r.Client.Create(ctx, immutableService)
			if err != nil {
				log.Error(err, "Failed to create new immutable Service", "Service.Name", immutableService.Name)
				return ctrl.Result{}, err
			}

			// Immutable Service created successfully - don't requeue
			return ctrl.Result{}, nil
		} else if err != nil {
			log.Error(err, "Failed to get immutable Service")
			return ctrl.Result{}, err
		}
		// Immutable Service already exists - don't requeue
		log.Info("Skip reconcile: Immutable Service already exists", "Service.Name", immutableService.Name)
		return ctrl.Result{}, nil
	}

	// Check if the deployment already exists, if not create a new one
	var deployment appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: codiusService.Name, Namespace: os.Getenv("CODIUS_NAMESPACE")}, &deployment)
	if err != nil && errors.IsNotFound(err) {
		dep, err := deploymentForCR(&codiusService)
		if err != nil {
			log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}
		// Set Codius Service as the owner and controller
		if err := controllerutil.SetControllerReference(&codiusService, dep, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Client.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully - don't requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Check if the Service already exists, if not create a new one
	var service corev1.Service
	err = r.Get(ctx, types.NamespacedName{Name: codiusService.Labels["codius.org/service"], Namespace: os.Getenv("CODIUS_NAMESPACE")}, &service)
	if err != nil && errors.IsNotFound(err) {
		ser := serviceForCR(&codiusService)
		// Set Codius Service instance as the owner and controller
		if err := controllerutil.SetControllerReference(&codiusService, ser, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.Client.Create(ctx, ser)
		if err != nil {
			log.Error(err, "Failed to create new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service.")
		return ctrl.Result{}, err
	}

	codiusService.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	codiusService.Status.UnavailableReplicas = deployment.Status.UnavailableReplicas
	if service.Annotations["codius.org/last-request-time"] != "" {
		reqTime, err := time.Parse(time.RFC3339, service.Annotations["codius.org/last-request-time"])
		if err != nil {
			log.Error(err, "Failed to parse service last request time", "last-request-time", service.Annotations["codius.org/last-request-time"])
		} else {
			codiusService.Status.LastRequestTime = &metav1.Time{Time: reqTime}
		}
	}
	if err := r.Status().Update(ctx, &codiusService); err != nil {
		log.Error(err, "Failed to update Status")
		return ctrl.Result{}, err
	}
	// update corresponding mutable services' status
	var mutableServices v1alpha1.ServiceList
	if err := r.List(ctx, &mutableServices, client.MatchingLabels{
		"codius.org/service":   codiusService.Labels["codius.org/service"],
		"codius.org/immutable": "false",
	}); err != nil {
		log.Error(err, "unable to list mutable Services")
		return ctrl.Result{}, err
	}
	for _, svc := range mutableServices.Items {
		svc.Status = codiusService.Status
		if err := r.Status().Update(ctx, &svc); err != nil {
			log.Error(err, "Failed to update mutable Service Status", "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
	}

	if codiusService.Status.LastRequestTime == nil || codiusService.Status.LastRequestTime.Add(time.Minute).Before(time.Now()) {
		if *deployment.Spec.Replicas >= int32(1) {
			replicas := int32(0)
			deployment.Spec.Replicas = &replicas
			if err := r.Update(ctx, &deployment); err != nil {
				log.Error(err, "Failed to scale down Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		if *deployment.Spec.Replicas == int32(0) {
			replicas := int32(1)
			deployment.Spec.Replicas = &replicas
			if err := r.Update(ctx, &deployment); err != nil {
				log.Error(err, "Failed to scale up Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				return ctrl.Result{}, err
			}
		}
		// Requeue a minute after the last request to try to scale down
		return ctrl.Result{
			RequeueAfter: time.Until(codiusService.Status.LastRequestTime.Add(time.Minute)),
		}, nil
	}

	// Deployment and Service already exist - don't requeue
	log.Info("Skip reconcile: Deployment and Service already exist", "Namespace", os.Getenv("CODIUS_NAMESPACE"), "Deployment.Name", deployment.Name, "Service.Name", service.Name)
	return ctrl.Result{}, nil
}

func deploymentForCR(cr *v1alpha1.Service) (*appsv1.Deployment, error) {
	labels := labelsForCR(cr)
	containers := make([]corev1.Container, len(cr.Spec.Containers))
	for i, container := range cr.Spec.Containers {
		envVars := make([]corev1.EnvVar, len(container.Env))
		for j, env := range container.Env {
			value := env.Value
			if env.ValueFrom != nil {
				value, _ = cr.SecretData[env.ValueFrom.SecretKeyRef.Key]
			}
			envVars[j] = corev1.EnvVar{
				Name:  env.Name,
				Value: value,
			}
		}
		containers[i] = corev1.Container{
			Name:           container.Name,
			Image:          container.Image,
			Command:        container.Command,
			Args:           container.Args,
			WorkingDir:     container.WorkingDir,
			Env:            envVars,
			LivenessProbe:  container.LivenessProbe,
			ReadinessProbe: container.ReadinessProbe,
			StartupProbe:   container.StartupProbe,
		}
	}

	automountServiceAccountToken := false
	enableServiceLinks := false

	var pRuntimeClassName *string
	runtimeClassName := os.Getenv("RUNTIME_CLASS_NAME")
	if runtimeClassName != "" {
		pRuntimeClassName = &runtimeClassName
	}
	ips, err := net.LookupHost(os.Getenv("CODIUS_HELLO_SVC_URL"))
	if err != nil {
		return nil, err
	}
	initCommand := fmt.Sprintf("while wget -T 1 --spider %s; do echo waiting for network policy enforcement; sleep 1; done", ips[0])
	replicas := int32(0)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: os.Getenv("CODIUS_NAMESPACE"),
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:                   containers,
					DNSPolicy:                    corev1.DNSDefault,
					EnableServiceLinks:           &enableServiceLinks,
					AutomountServiceAccountToken: &automountServiceAccountToken,
					RuntimeClassName:             pRuntimeClassName,
					InitContainers: []corev1.Container{
						{
							Image:   "busybox:1.31",
							Name:    "init-network-policy",
							Command: []string{"sh", "-c", initCommand},
						},
					},
				},
			},
		},
	}, nil
}

func serviceForCR(cr *v1alpha1.Service) *corev1.Service {
	labels := labelsForCR(cr)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			// Service names must be DNS-1035 labels
			// a DNS-1035 label must consist of lower case alphanumeric characters or '-',
			// start with an alphabetic character, and end with an alphanumeric character
			// (e.g. 'my-name',  or 'abc-123', regex used for validation is '[a-z]([-a-z0-9]*[a-z0-9])?'
			Name:      cr.Labels["codius.org/service"],
			Namespace: os.Getenv("CODIUS_NAMESPACE"),
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port: 80,
					// Currently unable to convert int32 to IntOrString...
					// https://godoc.org/k8s.io/apimachinery/pkg/util/intstr#IntOrString
					TargetPort: intstr.FromInt(int(cr.Spec.Port)),
				},
			},
		},
	}
}

// labelsForCR returns the labels for selecting the resources
// belonging to the given Codius Service name.
func labelsForCR(cr *v1alpha1.Service) map[string]string {
	return map[string]string{"codius.org/service": cr.Labels["codius.org/service"]}
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Service{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
