package utils

import (
	"github.com/oecp/rama/pkg/constants"
	"k8s.io/apimachinery/pkg/labels"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/pkg/errors"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	KubeAPIQPS   = 20.0
	KubeAPIBurst = 30
)

func BuildClusterConfig(rc *networkingv1.RemoteCluster) (*restclient.Config, error) {
	var (
		err         error
		clusterName = rc.ClusterName
		connConfig  = rc.Spec.ConnConfig
	)
	clusterConfig, err := clientcmd.BuildConfigFromFlags(connConfig.Endpoint, "")
	if err != nil {
		return nil, err
	}

	if len(connConfig.ClientCert) == 0 || len(connConfig.CABundle) == 0 || len(connConfig.ClientKey) == 0 {
		return nil, errors.Errorf("The connection data for cluster %s is missing", clusterName)
	}

	clusterConfig.CertData = connConfig.ClientCert
	clusterConfig.KeyData = connConfig.ClientKey
	clusterConfig.QPS = KubeAPIQPS
	clusterConfig.Burst = KubeAPIBurst

	return clusterConfig, nil
}

func SelectorClusterName(clusterName string) labels.Selector {
	s := labels.Set{
		constants.LabelCluster: clusterName,
	}
	return labels.SelectorFromSet(s)
}
