# Codius CRD Operator
> [Kubebuilder](https://book.kubebuilder.io/)-based operator for Codius [custom resource definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

### Dependencies

- [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) for [admission webhooks](https://book.kubebuilder.io/cronjob-tutorial/cert-manager.html)

### [Run](https://book.kubebuilder.io/quick-start.html#run-it-on-the-cluster)

```
sudo make docker-build docker-push IMG=<some-registry>/<project-name>:tag
make deploy IMG=<some-registry>/<project-name>:tag
```

### Environment Variables

Configure by patching the [controller manager deployment](config/manager/manager.yaml) with Kustomize.

#### CODIUS_AUTH_URL
* Type: String
* Description: Forward authorization URL of the Codius service ingress
* Default: random seed

#### CODIUS_CERT_SECRET
* Type: String
* Description: Kubernetes secret resource containing the Codius host's wildcard SSL certificate. The secret must be in the same namespace as the operator controller manager.
* Default: `codius-secret`

#### CODIUS_HOSTNAME
* Type: String
* Description: Hostname of the Codius host

#### CODIUS_HELLO_SVC_URL
* Type: String
* Description: URL for the internal [hello service](config/networkpolicy). Codius service deployment `initContainer`s will query the hello service to determine when the pod's [egress network policy](config/networkpolicy/networkpolicy.yaml) has been enforced.

#### CODIUS_NAMESPACE
* Type: String
* Description: Namespace in which to create deployments, services, and ingresses. The operator controller manager must have the necessary [permissions](config/rbac/role.yaml) in this namespace. This is also the namespace to which the [network policy](config/networkpolicy/networkpolicy.yaml) should be applied.

#### RUNTIME_CLASS_NAME
* Type: String
* Description: [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) to use for Codius service deployments' `runtimeClassName`
