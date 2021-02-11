module github.com/willemm/cluster-api-provider-scvmm

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/masterzen/winrm v0.0.0-20201030141608-56ca5c5f2380
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	sigs.k8s.io/cluster-api v0.3.12
	sigs.k8s.io/controller-runtime v0.5.14
)
