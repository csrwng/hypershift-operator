package oauthcert

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openshift/library-go/pkg/crypto"
)

const (
	OAuthDeploymentName = "oauth-openshift"
	CertHashAnnotation  = "hypershift.openshift.io/router-cert-hash"
)

type OAuthCertSyncer struct {
	// Client is a client that allows access to the management cluster
	Client kubeclient.Interface

	// Log is the logger for this controller
	Log logr.Logger

	// Namespace is the namespace where the control plane of the cluster
	// lives on the management server
	Namespace string

	// SecretLister is a lister for target cluster secrets
	SecretLister corelisters.SecretLister
}

func (o *OAuthCertSyncer) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	controllerLog := o.Log.WithValues("secret", req.NamespacedName.String())

	// Ignore any secret that is not kube-system/kubeadmin
	if req.Namespace != "openshift-ingress" || req.Name != "router-certs-default" {
		return ctrl.Result{}, nil
	}

	controllerLog.Info("Begin reconciling")

	secret, err := o.SecretLister.Secrets("openshift-ingress").Get("router-certs-default")
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{}, nil
	}
	hash, err := calculateHash(secret.Data)
	if err != nil {
		return ctrl.Result{}, err
	}
	oauthDeployment, err := o.Client.AppsV1().Deployments(o.Namespace).Get(OAuthDeploymentName, metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}
	updateNeeded := false
	if oauthDeployment.Spec.Template.ObjectMeta.Annotations == nil {
		oauthDeployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	currentValue := oauthDeployment.Spec.Template.ObjectMeta.Annotations[CertHashAnnotation]
	if currentValue != hash {
		oauthDeployment.Spec.Template.ObjectMeta.Annotations[CertHashAnnotation] = hash
		updateNeeded = true
		controllerLog.Info("An update is needed")
	}
	if !updateNeeded {
		return ctrl.Result{}, nil
	}

	// Generate new serving cert based on router CA
	ca, err := crypto.GetCAFromBytes(secret.Data["tls.crt"], secret.Data["tls.key"])
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get CA from router CA secret: %v", err)
	}
	controllerLog.Info("Obtained CA from router-certs-default")
	oauthConfig, err := o.Client.CoreV1().ConfigMaps(o.Namespace).Get("oauth-openshift", metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get oauth server config: %v", err)
	}
	controllerLog.Info("The current external address", "address", oauthConfig.Data["externalAddress"])
	hostnames := sets.NewString(oauthConfig.Data["externalAddress"])
	cert, err := ca.MakeServerCert(hostnames, 0)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to make certificate: %v", err)
	}

	certBytes, keyBytes, err := cert.GetPEMBytes()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to encode certificate: %v", err)
	}
	fmt.Println("Generated new server cert")
	fmt.Println(string(certBytes))
	fmt.Printf("\n\n")
	fmt.Printf("Generated new server key")
	fmt.Println(string(keyBytes))
	fmt.Printf("\n\n")
	oauthSecret, err := o.Client.CoreV1().Secrets(o.Namespace).Get("oauth-openshift", metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get oauth server secret: %v", err)
	}
	oauthSecret.Data["server.crt"] = certBytes
	oauthSecret.Data["server.key"] = keyBytes
	controllerLog.Info("About to update oauth secret")
	if _, err = o.Client.CoreV1().Secrets(o.Namespace).Update(oauthSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update oauth secret: %v", err)
	}

	controllerLog.Info("Updating Outh Server deployment")
	_, err = o.Client.AppsV1().Deployments(o.Namespace).Update(oauthDeployment)
	return ctrl.Result{}, err
}

func calculateHash(data map[string][]byte) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(b)), nil
}
