module github.com/willemm/cluster-api-provider-scvmm

go 1.16

replace github.com/masterzen/winrm v0.0.0-20211231115050-232efb40349e => github.com/willemm/winrm v0.0.0-20220606164341-4d91010d9d78

require (
	github.com/go-logr/logr v1.2.2
	github.com/hirochachacha/go-smb2 v1.0.10
	github.com/masterzen/winrm v0.0.0-20211231115050-232efb40349e
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	sigs.k8s.io/cluster-api v1.1.0
	sigs.k8s.io/controller-runtime v0.11.0
)
