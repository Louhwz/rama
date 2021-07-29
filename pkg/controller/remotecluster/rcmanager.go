package remotecluster

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/klog"

	apiv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/utils"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const UserAgentName = "Cluster-Controller"

type Manager struct {
	clusterID           uint32
	clusterName         string
	inClusterKubeClient kubeclientset.Interface
	kubeClient          *kubeclientset.Clientset
	ramaClient          *versioned.Clientset
	kubeInformerFactory informers.SharedInformerFactory
	ramaInformerFactory externalversions.SharedInformerFactory
	nodeLister          corev1.NodeLister
	nodeSynced          cache.InformerSynced
	subnetLister        listers.SubnetLister
	subnetSynced        cache.InformerSynced
	ipLister            listers.IPInstanceLister
	ipSynced            cache.InformerSynced
	clusterStatus       *apiv1.RemoteClusterStatus
}

func NewRemoteClusterManager(client kubeclientset.Interface, rc *apiv1.RemoteCluster) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			klog.Warningf("Panic hanppened. Maybe wrong kube config. err=%v", err)
		}
	}()

	config, err := utils.BuildClusterConfig(client, rc)
	if err != nil {
		return nil, err
	}
	rcManager := &Manager{
		clusterID:   rc.Spec.ClusterID,
		clusterName: rc.Spec.ClusterName,
	}
	config.Timeout = time.Duration(rc.Spec.ConnConfig.Timeout) * time.Second

	rcManager.kubeClient = kubeclientset.NewForConfigOrDie(config)
	rcManager.ramaClient = versioned.NewForConfigOrDie(restclient.AddUserAgent(config, UserAgentName))

	rcManager.kubeInformerFactory = informers.NewSharedInformerFactory(rcManager.kubeClient, 0)
	rcManager.ramaInformerFactory = externalversions.NewSharedInformerFactory(rcManager.ramaClient, 0)
	rcManager.nodeLister = rcManager.kubeInformerFactory.Core().V1().Nodes().Lister()
	rcManager.nodeSynced = rcManager.kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced
	rcManager.subnetLister = rcManager.ramaInformerFactory.Networking().V1().Subnets().Lister()
	rcManager.subnetSynced = rcManager.ramaInformerFactory.Networking().V1().Subnets().Informer().HasSynced
	rcManager.ipLister = rcManager.ramaInformerFactory.Networking().V1().IPInstances().Lister()
	rcManager.ipSynced = rcManager.ramaInformerFactory.Networking().V1().IPInstances().Informer().HasSynced

	nodeInformer := rcManager.kubeInformerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nil,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	subnetInformer := rcManager.ramaInformerFactory.Networking().V1().RemoteSubnets().Informer()
	subnetInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nil,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	ipInformer := rcManager.ramaInformerFactory.Networking().V1().IPInstances().Informer()
	ipInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nil,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	rcManager.inClusterKubeClient = client
	return rcManager, nil
}

func (m *Manager) getClusterHealthStatus() (*apiv1.RemoteClusterStatus, error) {
	clusterStatus := &apiv1.RemoteClusterStatus{}

	body, err := m.kubeClient.DiscoveryClient.RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).Raw()
	if err != nil {
		runtime.HandleError(errors.Wrapf(err, "Cluster Health Check failed for cluster %v", m.clusterID))
		m.clusterStatus.Conditions = append(m.clusterStatus.Conditions)
	} else {
		if !strings.EqualFold(string(body), "ok") {

		} else {

		}
	}
	return clusterStatus, nil
}

func (m *Manager) runNodeWorker() {

}

func (m *Manager) runSubnetWorker() {

}

func (m *Manager) runIPInstanceWorker() {

}
