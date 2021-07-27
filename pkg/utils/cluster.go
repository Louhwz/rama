package utils

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeclientset "k8s.io/client-go/kubernetes"

	v1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	restclient "k8s.io/client-go/rest"
)

const (
	KubeAPIQPS   = 20.0
	KubeAPIBurst = 30

	CAKey         = "ca"
	TokenKey      = "token"
	ClientCertKey = "client.crt"
	ClientKeyKey  = "client.key"
)

func BuildClusterConfig(client kubeclientset.Interface, rc *v1.RemoteCluster) (*restclient.Config, error) {
	var (
		err         error
		secret      = &apiv1.Secret{}
		clusterName = rc.Spec.ClusterName
		connConfig  = rc.Spec.ConnConfig
	)
	clusterConfig, err := clientcmd.BuildConfigFromFlags(connConfig.Endpoint, "")
	if err != nil {
		return nil, err
	}

	// todo check
	secret, err = client.CoreV1().Secrets(rc.ObjectMeta.Namespace).Get(context.TODO(), connConfig.SecretRef, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if ca, caFound := secret.Data[CAKey]; !caFound || len(ca) == 0 {
		return nil, errors.Errorf("The ca for cluster %s is missing", clusterName)
	} else {
		clusterConfig.CAData = ca
	}
	token, tokenFound := secret.Data[TokenKey]
	clientCert, clientCertFound := secret.Data[ClientCertKey]
	clientKey, clientKeyFound := secret.Data[ClientKeyKey]
	if (!tokenFound || len(token) == 0) && (!clientCertFound || len(clientCert) == 0 || !clientKeyFound || len(clientKey) == 0) {
		return nil, errors.Errorf("The secret for cluster %s is missing token and client key-pair data", clusterName)
	}

	if tokenFound && len(token) > 0 {
		clusterConfig.BearerToken = string(token)
	} else {
		clusterConfig.CertData = clientCert
		clusterConfig.KeyData = clientKey
	}

	clusterConfig.QPS = KubeAPIQPS
	clusterConfig.Burst = KubeAPIBurst
	return clusterConfig, nil
}
