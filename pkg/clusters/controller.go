package clusters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/remote"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Cluster add the cluster name to the standard controller-runtime Cluster
type Cluster struct {
	cluster.Cluster
	Name types.NamespacedName
	stop context.CancelFunc
}

// ClusterAccessor allow multi-cluster controllers to get the Cluster accociated with a reconsile request
type ClusterAccessor interface {
	GetCluster(name string) (*Cluster, error)
}

// ManagmentController is
type ManagmentController interface {
	ClusterAccessor
	Source(kind client.Object) source.Source
	WorkloadClusterController(name string, options controller.Options) (controller.Controller, error)
}

type managementCluster struct {
	client   client.Client
	manager  manager.Manager
	logger   logr.Logger
	clusters map[types.NamespacedName]*Cluster
	created  chan *Cluster
}

func (mc *managementCluster) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var resource capi.Cluster

	err := mc.client.Get(ctx, req.NamespacedName, &resource)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	watch := err == nil &&
		resource.DeletionTimestamp == nil &&
		resource.Status.ControlPlaneReady &&
		!resource.Spec.Paused

	rc, ok := mc.clusters[req.NamespacedName]
	if watch == ok { // noop: desired == actual
		return reconcile.Result{}, nil
	}

	//mc.logger.Info("reconsile cluster", "cluster", req.NamespacedName, "watch", watch)

	if !watch {
		rc.stop()
		delete(mc.clusters, req.NamespacedName)
		return reconcile.Result{}, nil
	}

	// add clusters
	config, err := remote.RESTConfig(ctx, mc.client, req.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}
	cluster, err := cluster.New(config)
	if err != nil {
		return reconcile.Result{}, err
	}
	ctx, stop := context.WithCancel(context.TODO())

	go func() {
		mc.logger.Info("starting cluster", "cluster", req.NamespacedName)
		if err := cluster.Start(ctx); err != nil {
			mc.logger.Error(err, "could not start cluster", "cluster", fmt.Sprintf("%s/%s", req.Namespace, req.Name))
		}
	}()

	rc = &Cluster{
		Cluster: cluster,
		Name:    req.NamespacedName,
		stop:    stop,
	}
	mc.clusters[req.NamespacedName] = rc
	mc.created <- rc

	return reconcile.Result{}, nil
}

func (mc *managementCluster) Source(kind client.Object) source.Source {
	return starter(func(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
		go func() {
			for cluster := range mc.created {
				src := RemoteKind{Cluster: cluster.Name.Namespace + ":" + cluster.Name.Name, SyncingSource: source.NewKindWithCache(kind, cluster.GetCache())}
				if err := src.Start(ctx, handler, queue, predicates...); err != nil {
					mc.logger.Error(err, "could not start source")
				}
			}
		}()
		return nil
	})
}

func (mc *managementCluster) WorkloadClusterController(name string, options controller.Options) (controller.Controller, error) {
	return controller.New(name, mc.manager, options)
}

func (mc *managementCluster) GetCluster(name string) (*Cluster, error) {
	nsn := strings.Split(name, ":")
	cluster, ok := mc.clusters[types.NamespacedName{Namespace: nsn[0], Name: nsn[1]}]
	if !ok {
		return nil, errors.New("cluster not found")
	}
	return cluster, nil
}

// NewManagementController create a controller that watch a management cluster for clusters' lifecycle
func NewManagementController(m manager.Manager, predicates ...predicate.Predicate) (ManagmentController, error) {
	m.GetLogger()
	ctrl := &managementCluster{
		client:   m.GetClient(),
		manager:  m,
		logger:   m.GetLogger().WithValues("controller", "management"),
		clusters: map[types.NamespacedName]*Cluster{},
		created:  make(chan *Cluster),
	}

	err := builder.ControllerManagedBy(m).
		For(&capi.Cluster{}, builder.WithPredicates(predicates...)).
		Complete(ctrl)
	return ctrl, err
}
