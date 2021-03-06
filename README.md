# Codius Operator
> [Kubebuilder](https://book.kubebuilder.io/)-based operator for Codius [custom resource definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)

![](https://github.com/codius/codius-operator/workflows/Docker%20CI/badge.svg)

### Dependencies

- [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) for [admission webhooks](https://book.kubebuilder.io/cronjob-tutorial/cert-manager.html)

### [Run](https://book.kubebuilder.io/quick-start.html#run-it-on-the-cluster)

```
sudo make docker-build docker-push IMG=<some-registry>/<project-name>:tag
make deploy IMG=<some-registry>/<project-name>:tag
```

### Environment Variables

Configure by patching the [controller manager deployment](config/manager/manager.yaml) with Kustomize.

#### CODIUS_HOSTNAME
* Type: String
* Description: Hostname of the Codius host

#### CODIUS_HELLO_SVC_URL
* Type: String
* Description: URL for the internal [hello service](config/networkpolicy). Codius service deployment `initContainer`s will query the hello service to determine when the pod's [egress network policy](config/networkpolicy/networkpolicy.yaml) has been enforced.

#### CODIUS_WEB_URL
* Type: String
* Description: URL of the [Codius web](https://github.com/codius/codius-web/) frontend.

#### CODIUS_NAMESPACE
* Type: String
* Description: Namespace in which to create deployments, services, and ingresses. The operator controller manager must have the necessary [permissions](config/rbac/role.yaml) in this namespace. This is also the namespace to which the [network policy](config/networkpolicy/networkpolicy.yaml) should be applied.

#### RECEIPT_VERIFIER_URL
* Type: String
* Description: URL of the [receipt verifier](https://github.com/coilhq/receipt-verifier/) with which to deduct paid balances.

#### REQUEST_PRICE
* Type: Number
* Description: The amount required to have been paid to serve a request. Denominated in the host's asset (code and scale).

#### RUNTIME_CLASS_NAME
* Type: String
* Description: [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) to use for Codius service deployments' `runtimeClassName`

#### SERVICE_PRICE
* Type: Number
* Description: The amount required to have been paid to create a service. Denominated in the host's asset (code and scale).

### API Documentation

#### `PUT /services/{ID}`

Create a [Codius service](https://godoc.org/github.com/codius/codius-operator/api/v1alpha1#Service)

##### Request Body:

* Type: [Object](https://godoc.org/github.com/codius/codius-operator/servers#Service)

| Field Name | Type     | Description              |
|------------|----------|--------------------------|
| [spec](https://godoc.org/github.com/codius/codius-operator/api/v1alpha1#ServiceSpec) | Object | An object containing details for your service.|
| [secretData](https://godoc.org/github.com/codius/codius-operator/api/v1alpha1#Service) | Object | An object containing private variables you want to pass to the host, such as an AWS key.|

#### `GET /services/{ID}`

Retrieve the specified [Codius service](https://godoc.org/github.com/codius/codius-operator/api/v1alpha1#Service)
