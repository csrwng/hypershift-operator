package oauthcert

import (
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift-hive/hypershift-operator/pkg/cmd/operator"
	"github.com/openshift-hive/hypershift-operator/pkg/controllers"
)

func Setup(cfg *operator.ControlPlaneOperatorConfig) error {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(cfg.TargetKubeClient(), controllers.DefaultResync, informers.WithNamespace("openshift-ingress"))
	cfg.Manager().Add(manager.RunnableFunc(func(stopCh <-chan struct{}) error {
		informerFactory.Start(stopCh)
		return nil
	}))
	secrets := informerFactory.Core().V1().Secrets()
	reconciler := &OAuthCertSyncer{
		Client:       cfg.KubeClient(),
		Namespace:    cfg.Namespace(),
		SecretLister: secrets.Lister(),
		Log:          cfg.Logger().WithName("OAuthCertSyncer"),
	}
	c, err := controller.New("oauth-cert-syncer", cfg.Manager(), controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}
	if err := c.Watch(&source.Informer{Informer: secrets.Informer()}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	return nil
}
