package clusters

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type decorator struct {
	cluster string
	inner   handler.EventHandler
}

func (d decorator) Create(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.Object.SetClusterName(d.cluster)
	d.inner.Create(e, q)
}
func (d decorator) Update(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.ObjectNew.SetClusterName(d.cluster)
	e.ObjectOld.SetClusterName(d.cluster)
	d.inner.Update(e, q)
}
func (d decorator) Delete(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.Object.SetClusterName(d.cluster)
	d.inner.Delete(e, q)

}
func (d decorator) Generic(e event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.Object.SetClusterName(d.cluster)
	d.inner.Generic(e, q)
}

type starter func(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error

func (f starter) Start(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
	return f(ctx, handler, queue, predicates...)
}
