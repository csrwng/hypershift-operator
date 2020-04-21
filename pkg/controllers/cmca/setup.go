package cmca

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift-hive/hypershift-operator/pkg/cmd/operator"
	"github.com/openshift-hive/hypershift-operator/pkg/controllers"
)

const (
	ManagedConfigNamespace                 = "openshift-config-managed"
	ControllerManagerAdditionalCAConfigMap = "controller-manager-additional-ca"
)

func Setup(cfg *operator.ControlPlaneOperatorConfig) error {
	if err := setupConfigMapObserver(cfg); err != nil {
		return err
	}
	return nil
}

func setupConfigMapObserver(cfg *operator.ControlPlaneOperatorConfig) error {
	informerFactory := cfg.TargetKubeInformersForNamespace(ManagedConfigNamespace)
	configMaps := informerFactory.Core().V1().ConfigMaps()
	reconciler := &ManagedCAObserver{
		InitialCA:      cfg.InitialCA(),
		Client:         cfg.KubeClient(),
		TargetCMLister: configMaps.Lister(),
		Namespace:      cfg.Namespace(),
		Log:            cfg.Logger().WithName("ManagedCAObserver"),
	}
	c, err := controller.New("ca-configmap-observer", cfg.Manager(), controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}
	if err := c.Watch(&source.Informer{Informer: configMaps.Informer()}, controllers.NamedResourceHandler(RouterCAConfigMap, ServiceCAConfigMap)); err != nil {
		return err
	}
	return nil
}
