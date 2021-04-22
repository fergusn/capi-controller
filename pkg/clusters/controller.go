package clusters

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
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
	AddRemote(nsn types.NamespacedName, config *rest.Config) error
}

type managementCluster struct {
	client   client.Client
	manager  manager.Manager
	logger   logr.Logger
	clusters map[types.NamespacedName]*Cluster
	created  chan *Cluster
	kind     client.Object
}

func (mc *managementCluster) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var resource capi.Cluster

	mc.logger.Info("reconsile", "namespace", req.Namespace, "name", req.Name)

	err := mc.client.Get(ctx, req.NamespacedName, &resource)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	watch := err == nil &&
		resource.DeletionTimestamp == nil &&
		resource.Status.ControlPlaneInitialized &&
		!resource.Spec.Paused

	rc, ok := mc.clusters[req.NamespacedName]
	if watch == ok { // noop: desired == actual
		return reconcile.Result{}, nil
	}

	//mc.logger.Info("reconsile cluster", "cluster", req.NamespacedName, "watch", watch)

	if !watch {
		mc.logger.Info("deleting cluster", "cluster", req.NamespacedName)
		rc.stop()
		delete(mc.clusters, req.NamespacedName)
		mc.created <- rc
		return reconcile.Result{}, nil
	}

	// add clusters
	config, err := remote.RESTConfig(ctx, mc.client, req.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Create cluster
	cluster, err := cluster.New(config, func(opts *cluster.Options) {
		// Set scheme of cluster to that of the management cluster
		opts.Scheme = mc.manager.GetScheme()
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	gvks, _, err := mc.manager.GetScheme().ObjectKinds(mc.kind)
	if err != nil || len(gvks) != 1 {
		log.Fatal("could not get kind metadata")
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvks[0].Group,
		Kind:    gvks[0].Kind + "List",
		Version: gvks[0].Version,
	})
	if err := cluster.GetClient().List(ctx, list); err != nil {
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

func (mc *managementCluster) AddRemote(nsn types.NamespacedName, config *rest.Config) error {
	cluster, err := cluster.New(config, func(opts *cluster.Options) {
		// Set scheme of cluster to that of the management cluster
		opts.Scheme = mc.manager.GetScheme()
	})
	if err != nil {
		return err
	}

	ctx, stop := context.WithCancel(context.TODO())

	go func() {
		mc.logger.Info("starting cluster", "cluster", nsn)
		if err := cluster.Start(ctx); err != nil {
			mc.logger.Error(err, "could not start cluster", "cluster", fmt.Sprintf("%s/%s", nsn.Namespace, nsn.Name))
		}
	}()

	rc := &Cluster{
		Cluster: cluster,
		Name:    nsn,
		stop:    stop,
	}
	mc.clusters[nsn] = rc
	mc.created <- rc
	return nil
}

func (mc *managementCluster) Source(kind client.Object) source.Source {
	mc.kind = kind
	return starter(func(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
		go func() {
			for cluster := range mc.created {
				mc.logger.Info("watching cluster %s/%s", cluster.Name.Namespace, cluster.Name.Name)
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
		gvk := (&capi.Cluster{}).GetObjectKind().GroupVersionKind()
		return nil, apierrors.NewNotFound(
			schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, nsn[1],
		)
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
		created:  make(chan *Cluster, 20),
	}

	err := builder.ControllerManagedBy(m).
		For(&capi.Cluster{}, builder.WithPredicates(predicates...)).
		Complete(ctrl)
	return ctrl, err
}
