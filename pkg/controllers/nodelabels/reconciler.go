package nodelabels

import (
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var requiredLabels = map[string]string{
	"node-role.kubernetes.io/worker": "",
	"node-role.kubernetes.io/master": "",
}

type NodeLabels struct {
	Lister     corev1lister.NodeLister
	KubeClient kubeclient.Interface
	Log        logr.Logger
}

func (a *NodeLabels) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := a.Log.WithValues("node", req.NamespacedName.String())
	logger.Info("Start reconcile")
	node, err := a.Lister.Get(req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hasRequiredLabels(node) {
		return ctrl.Result{}, nil
	}

	for k := range requiredLabels {
		if _, hasLabel := node.Labels[k]; hasLabel {
			continue
		}
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels[k] = requiredLabels[k]
	}

	logger.Info("Updating node labels")
	_, err = a.KubeClient.CoreV1().Nodes().Update(node)
	if err != nil {
		a.Log.Error(err, "failed to update node labels")
	}
	return ctrl.Result{}, err
}

func hasRequiredLabels(node *corev1.Node) bool {
	for k := range requiredLabels {
		if _, hasLabel := node.Labels[k]; !hasLabel {
			return false
		}
	}
	return true
}
