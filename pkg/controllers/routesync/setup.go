package routesync

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	routeinformers "github.com/openshift/client-go/route/informers/externalversions"

	"github.com/openshift-hive/hypershift-operator/pkg/cmd/operator"
	"github.com/openshift-hive/hypershift-operator/pkg/controllers"
)

func Setup(cfg *operator.ControlPlaneOperatorConfig) error {
	targetClient, err := routeclient.NewForConfig(cfg.TargetConfig())
	if err != nil {
		return err
	}

	hostClient, err := routeclient.NewForConfig(cfg.Config())
	if err != nil {
		return err
	}

	targetInformerFactory := routeinformers.NewSharedInformerFactory(targetClient, controllers.DefaultResync)
	cfg.Manager().Add(manager.RunnableFunc(func(stopCh <-chan struct{}) error {
		targetInformerFactory.Start(stopCh)
		return nil
	}))
	targetRoutes := targetInformerFactory.Route().V1().Routes()

	hostInformerFactory := routeinformers.NewSharedInformerFactoryWithOptions(hostClient, controllers.DefaultResync, routeinformers.WithNamespace(cfg.Namespace()))
	cfg.Manager().Add(manager.RunnableFunc(func(stopCh <-chan struct{}) error {
		hostInformerFactory.Start(stopCh)
		return nil
	}))
	hostRoutes := hostInformerFactory.Route().V1().Routes()

	reconciler := &RouteSyncReconciler{
		HostClient:   hostClient,
		Namespace:    cfg.Namespace(),
		TargetLister: targetRoutes.Lister(),
		HostLister:   hostRoutes.Lister(),
		Log:          cfg.Logger().WithName("RouteSync"),
	}
	c, err := controller.New("route-sync", cfg.Manager(), controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}
	if err := c.Watch(&source.Informer{Informer: targetRoutes.Informer()}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	return nil
}
