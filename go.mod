module github.com/willemm/cluster-api-provider-scvmm

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/hirochachacha/go-smb2 v1.0.10
	github.com/masterzen/winrm v0.0.0-20210623064412-3b76017826b0 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/pkg/errors v0.9.1
	github.com/willemm/winrm v0.0.0-20210223164918-ea3696aa9375
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/cluster-api v1.0.0
	sigs.k8s.io/controller-runtime v0.10.2
)
