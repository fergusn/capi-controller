module github.com/fergusn/capi-controller

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/cluster-api v0.3.11-0.20210108151854-953ad253ee2b
	sigs.k8s.io/controller-runtime v0.7.1-0.20210108153454-66537ca5b743
)

replace sigs.k8s.io/controller-runtime => github.com/fergusn/controller-runtime v0.8.1-1
