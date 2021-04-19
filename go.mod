module github.com/fergusn/capi-controller

go 1.15

require (
	github.com/go-logr/logr v0.4.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	sigs.k8s.io/cluster-api v0.3.11-0.20210108151854-953ad253ee2b
	sigs.k8s.io/controller-runtime v0.9.0-alpha.1
)

replace sigs.k8s.io/controller-runtime => github.com/fergusn/controller-runtime v0.9.0-alpha.1
