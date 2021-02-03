package main

import (
	"context"
	"log"

	"github.com/fergusn/capi-controller/pkg/clusters"
	"k8s.io/apimachinery/pkg/runtime"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	corev1 "k8s.io/client-go/kubernetes/scheme"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

func main() {
	scheme := runtime.NewScheme()

	capiv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	mgr, err := manager.New(controller.GetConfigOrDie(), manager.Options{
		LeaderElection: false,
		Scheme:         scheme,
	})
	if err != nil {
		log.Fatal(err)
	}
	mc, err := clusters.NewManagementController(mgr)
	if err != nil {
		log.Fatal(err)
	}

	err = buildController(mc)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(mgr.Start(context.Background()))
}
