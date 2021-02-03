package main

import (
	"context"

	"github.com/fergusn/capi-controller/pkg/clusters"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconsiler struct {
	clusters clusters.ClusterAccessor
}

func (r *reconsiler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	cluster, err := r.clusters.GetCluster(req)
	if err != nil {
		return reconcile.Result{}, err
	}
	client := cluster.GetClient()

	svc := v1.Service{}

	err = client.Get(ctx, req.NamespacedName, &svc)
	if err != nil {
		return reconcile.Result{}, err
	}

	if cluster.Name.Name == "sandbox" {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func buildController(mc clusters.ManagmentController) error {
	ctrl, err := mc.WorkloadClusterController("test", controller.Options{
		Reconciler: &reconsiler{clusters: mc},
	})
	if err != nil {
		return err
	}
	return ctrl.Watch(mc.Source(&v1.Service{}), &handler.EnqueueRequestForObject{})
}
