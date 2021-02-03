package clusters

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type limiterDecorator struct {
	workqueue.RateLimiter
	foget func(interface{})
}

func (l *limiterDecorator) Forget(item interface{}) {
	l.foget(item)
	l.RateLimiter.Forget(item)
}

type queueDecorator struct {
	workqueue.RateLimitingInterface
	add func(interface{})
}

func (q *queueDecorator) AddAfter(item interface{}, duration time.Duration) {
	q.add(item)
	q.RateLimitingInterface.AddAfter(item, duration)
}
func (q *queueDecorator) Add(item interface{}) {
	q.add(item)
	q.RateLimitingInterface.Add(item)
}
func (q *queueDecorator) AddRateLimited(item interface{}) {
	q.add(item)
	q.RateLimitingInterface.AddRateLimited(item)
}

type starter func(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error

func (f starter) Start(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
	return f(ctx, handler, queue, predicates...)
}
