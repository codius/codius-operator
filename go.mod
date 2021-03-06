module github.com/codius/codius-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/julienschmidt/httprouter v1.2.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/rs/cors v1.7.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/kubectl v0.0.0-20191219154910-1528d4eea6dd
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.5.4 // indirect
)
