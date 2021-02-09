package clusters

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type RemoteKind struct {
	source.SyncingSource
	Cluster string
}

func (ks *RemoteKind) Start(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, prct ...predicate.Predicate) error {
	return ks.SyncingSource.Start(ctx, decorator{cluster: ks.Cluster, inner: handler}, queue, prct...)
}
